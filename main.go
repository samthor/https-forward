package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

var (
	flagHSTS    = flag.Duration("hsts", time.Hour*24, "duration for HSTS header")
	flagTimeout = flag.Duration("timeout", time.Minute, "timeout for proxied hosts")
	flagLog     = flag.Bool("log", false, "set to log all network requests")
	flagEmail   = flag.String("email", "", "email to provide to LE")
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

	config := &configHolder{
		transport: &http.Transport{
			ResponseHeaderTimeout: *flagTimeout,
		},
	}
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
		if *flagLog {
			log.Printf("allowing host: %v", host)
		}
		return nil
	}

	hostRouter := func(w http.ResponseWriter, r *http.Request) {
		if *flagLog {
			log.Printf("req: https://%v%v (remote=%v)", r.Host, r.URL.Path, r.RemoteAddr)
		}

		host := stripPort(r.Host)
		hc, ok := config.For(host)
		if !ok {
			if *flagLog {
				log.Printf("bad host lookup: %v", host)
			}

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
		if r.URL.Path == "/" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1" />
<meta name="google" value="notranslate" />
</head>
<body>
<div align="center"><code>¯\_(ツ)_/¯</code></div>
<!-- powered by https://github.com/samthor/https-forward -->
</body>`)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}

	// setup normal certManager
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: hostPolicy,
		Email:      *flagEmail,
	}
	if *flagCache != "" {
		certManager.Cache = autocert.DirCache(*flagCache)
	}

	// configure tlsConfig to allow self-signed on "localhost" which comes from e.g. Cloudflare
	tlsConfig := certManager.TLSConfig()
	getCertificate := tlsConfig.GetCertificate
	tlsConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello.ServerName == "localhost" {
			if *flagLog {
				log.Printf("using self-signed cert")
			}
			return &selfSignedCert, nil
		}
		if *flagLog {
			log.Printf("tls init %+v", hello)
		}
		return getCertificate(hello)
	}

	// include certManager's HTTP in https?
	server := &http.Server{
		Addr:      ":https",
		Handler:   certManager.HTTPHandler(http.HandlerFunc(hostRouter)),
		TLSConfig: tlsConfig,
	}

	// must call this outside handler; _enables_ http-01
	autocertHTTPHandler := certManager.HTTPHandler(nil) // automatically redirects
	go func() {
		var h http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			if *flagLog {
				log.Printf("http req: http://%v%v (remote=%v)", r.Host, r.URL.Path, r.RemoteAddr)
			}
			autocertHTTPHandler.ServeHTTP(w, r)
		}
		log.Fatal(http.ListenAndServe(":http", h))
	}()

	log.Fatal(server.ListenAndServeTLS("", ""))
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}
