package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

var (
	flagHSTS = flag.Duration("hsts", time.Hour*24, "duration for HSTS header")
)

const (
	forwardedFor = "X-Forwarded-For"
)

func main() {
	var (
		flagConfig *string
		flagCache  *string
	)

	// configure flagConfig, in snap use common across revisions
	if snapCommon := os.Getenv("SNAP_COMMON"); snapCommon != "" {
		configPath := path.Join(snapCommon, "config")
		flagConfig = &configPath
	} else {
		flagConfig = flag.String("config", "/etc/https-forward", "config file to read")
	}

	// configure *flagCache, in SNAP mode just use its semi-permanent cache
	if snapData := os.Getenv("SNAP_DATA"); snapData != "" {
		cachePath := path.Join(snapData, "cache")
		flagCache = &cachePath
	} else {
		flagCache = flag.String("cache", "/tmp/autocert", "cert cache directory, blank for memory")
	}

	flag.Parse()
	log.Printf("config=%v, cache=%v", *flagConfig, *flagCache)

	config := &configHolder{config: make(map[string]hostConfig)}
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
		return nil
	}

	hostRouter := func(w http.ResponseWriter, r *http.Request) {
		host := stripPort(r.Host)
		hc, ok := config.For(host)
		if !ok {
			// should never get here: SSL cert should not be generated
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// set https-only
		sec := int((*flagHSTS).Seconds())
		w.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d; includeSubDomains", sec))

		// auth if needed
		if hc.auth {
			username, password, ok := r.BasicAuth()
			if !ok {
				v := fmt.Sprintf(`Basic realm="%s"`, host)
				w.Header().Set("WWW-Authenticate", v)
				http.Error(w, "", http.StatusUnauthorized)
				return
			}
			if allowed := hc.Allow(username, password); !allowed {
				http.Error(w, "", http.StatusForbidden)
				return
			}
		}

		// success
		if hc.proxy != nil {
			hc.proxy.ServeHTTP(w, r)
			return
		}

		// top-level domains don't do anything
		if r.URL.Path == "/" {
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<meta name="google" value="notranslate" />
</head>
<body>
<code>¯\_(ツ)_/¯</code>
</body>`)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}

	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: hostPolicy,
	}
	if *flagCache != "" {
		certManager.Cache = autocert.DirCache(*flagCache)
	}
	server := &http.Server{
		Addr:      ":https",
		Handler:   http.HandlerFunc(hostRouter),
		TLSConfig: certManager.TLSConfig(),
	}

	go func() {
		log.Fatal(http.ListenAndServe(":http", http.HandlerFunc(handleRedirect)))
	}()

	log.Fatal(server.ListenAndServeTLS("", ""))
}

func handleRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" || r.Method == "HEAD" {
		target := "https://" + stripPort(r.Host) + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusFound)
	} else {
		http.Error(w, "", http.StatusBadRequest)
	}
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}
