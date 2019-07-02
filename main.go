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
	"strings"
	"sync"
	"syscall"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var (
	flagHost   = flag.String("host", "hh.whistlr.info", "the hostname and subdomains to allow")
	flagConfig = flag.String("config", "/etc/https-forward", "config file to read")
)

const (
	forwardedFor = "X-Forwarded-For"
)

type configHolder struct {
	lock   sync.Mutex
	config map[string]string
}

func (ch *configHolder) For(host string) string {
	ch.lock.Lock()
	defer ch.lock.Unlock()
	return ch.config[host]
}

func (ch *configHolder) Read(f string) error {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}

	out := make(map[string]string)
	lines := bytes.Split(b, []byte("\n"))
	for _, line := range lines {
		comment := bytes.IndexRune(line, '#')
		if comment > -1 {
			line = line[:comment]
		}
		f := bytes.Fields(line)
		if len(f) != 2 || len(f[0]) == 0 {
			continue
		}

		out[string(f[0])] = string(f[1])
	}
	log.Printf("read config, got %d entries", len(out))

	ch.lock.Lock()
	defer ch.lock.Unlock()
	ch.config = out
	return nil
}

func candForSuffix(host, suffix string) string {
	if !strings.HasSuffix(host, "."+suffix) {
		return ""
	}
	return host[:len(host)-(len(suffix)+1)]
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
		if host == *flagHost {
			return nil
		} else if cand := candForSuffix(host, *flagHost); cand != "" {
			target := config.For(cand)
			if target == "" {
				return fmt.Errorf("unconfigured host: %v", host)
			}
			log.Printf("allowing: %v", host)
			return nil
		}
		return fmt.Errorf("disallowing host: %v", host)
	}

	hostRouter := func(w http.ResponseWriter, r *http.Request) {
		if r.Host == *flagHost {
			if r.URL.Path == "/" {
				fmt.Fprintf(w, `¯\_(ツ)_/¯`)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		cand := candForSuffix(r.Host, *flagHost)
		target := config.For(cand)
		if target == "" {
			// should never get here: SSL cert should not be generated
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// https for a day
		sec := 86400
		w.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d; includeSubDomains", sec))

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
