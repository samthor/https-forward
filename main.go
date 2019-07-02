package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var (
	flagConfig = flag.String("config", "/etc/https-forward", "config file to read")
)

const (
	forwardedFor = "X-Forwarded-For"
)

type configHolder struct {
	lock   sync.Mutex
	config map[string]string
}

func (ch *configHolder) For(host string) (string, bool) {
	ch.lock.Lock()
	defer ch.lock.Unlock()
	out, ok := ch.config[host]
	return out, ok
}

func (ch *configHolder) Read(f string) error {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}

	var host string

	out := make(map[string]string)
	lines := bytes.Split(b, []byte("\n"))
	for _, line := range lines {
		comment := bytes.IndexRune(line, '#')
		if comment > -1 {
			line = line[:comment]
		}
		f := bytes.Fields(line)

		// line like ".foo.bar.com", start new hostname
		if len(f) == 1 && f[0][0] == '.' {
			host = string(f[0])

			// insert a dummy config for "foo.bar.com"
			out[host[1:]] = ""
		}

		// otherwise we expect "blah localhost:8080"
		if len(f) != 2 || len(f[0]) == 0 {
			continue
		}
		out[string(f[0]) + host] = string(f[1])
	}
	log.Printf("read config, got %d entries", len(out))

	ch.lock.Lock()
	defer ch.lock.Unlock()
	ch.config = out
	return nil
}

func main() {
	flag.Parse()

	config := &configHolder{config: make(map[string]string)}
	err := config.Read(*flagConfig)
	if err != nil {
		log.Fatalf("could not read config: %v", err)
	}

	// listen to SIGHUP for config changes
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for range c {
			err := config.Read(*flagConfig)
			if err != nil {
				log.Fatalf("could not read config: %v", err)
			}
		}
	}()

	hostPolicy := func(c context.Context, host string) error {
		if _, ok := config.For(host); !ok {
			return fmt.Errorf("disallowing host: %v", host)
		}
		log.Printf("allowing: %v", host)
		return nil
	}

	hostRouter := func(w http.ResponseWriter, r *http.Request) {
		target, ok := config.For(r.Host)
		if !ok {
			// should never get here: SSL cert should not be generated
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// https for a day
		sec := 86400
		w.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d; includeSubDomains", sec))

		// top-level domains don't do anything
		if target == "" {
			if r.URL.Path == "/" {
				fmt.Fprintf(w, `¯\_(ツ)_/¯`)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				log.Printf("req: %v%+v (remote=%v)", r.Host, r.URL.Path, r.RemoteAddr)
				req.Host = target
				req.URL.Scheme = "http"
				req.URL.Host = target
				if _, ok := req.Header["User-Agent"]; !ok {
					req.Header.Set("User-Agent", "") // don't allow default value here
				}
			},
		}
		proxy.ServeHTTP(w, r)
	}

	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache("/tmp/autocert"),
		HostPolicy: hostPolicy,
	}
	server := &http.Server{
		Addr:    ":https",
		Handler: http.HandlerFunc(hostRouter),
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
			NextProtos:     []string{acme.ALPNProto},
		},
	}

	go func() {
		//h := certManager.HTTPHandler(nil)
		//log.Fatal(http.ListenAndServe(":http", h))
	}()

	log.Fatal(server.ListenAndServeTLS("", ""))
}
