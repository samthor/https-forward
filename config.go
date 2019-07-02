package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"sync"
)

type hostConfig struct {
	target   string
	auth     bool
	username string
	password string
	proxy    *httputil.ReverseProxy
}

func (hc *hostConfig) Allow(username, password string) bool {
	if hc.username != "" && hc.username != username {
		return false
	}
	if hc.password != "" && hc.password != password {
		return false
	}
	return true
}

type configHolder struct {
	lock   sync.Mutex
	config map[string]hostConfig
}

func (ch *configHolder) For(host string) (hostConfig, bool) {
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

	var suffix string

	out := make(map[string]hostConfig)
	lines := bytes.Split(b, []byte("\n"))
	for _, line := range lines {
		comment := bytes.IndexRune(line, '#')
		if comment > -1 {
			line = line[:comment]
		}
		f := bytes.Fields(line)

		// line like ".foo.bar.com", start new hostname group
		if len(f) == 1 && f[0][0] == '.' {
			suffix = string(f[0])
			host := suffix[1:]

			// insert a dummy config for "foo.bar.com"
			out[host] = hostConfig{}
			log.Printf("matched top-level: %s", host)
			continue
		}

		// otherwise we expect "blah localhost:8080 opt_user:and_pass"
		if len(f) > 3 || len(f) == 0 {
			continue
		}
		qualified := string(f[0]) + suffix
		hc := hostConfig{
			target: string(f[1]),
		}

		if len(f) > 2 {
			parts := bytes.SplitN(f[2], []byte{byte(':')}, 2)
			hc.username = string(parts[0])
			if len(parts) == 2 {
				hc.password = string(parts[1])
			}
			hc.auth = true
		}

		hc.proxy = buildProxy(hc.target, hc.auth)

		out[qualified] = hc
		log.Printf("matched: %s => %s", qualified, hc.target)
	}
	log.Printf("read config, got %d entries", len(out))

	ch.lock.Lock()
	defer ch.lock.Unlock()
	ch.config = out
	return nil
}

func buildProxy(target string, auth bool) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			log.Printf("req: %v%+v (remote=%v)", req.Host, req.URL.Path, req.RemoteAddr)
			req.Host = target
			req.URL.Scheme = "http"
			req.URL.Host = target

			if auth {
				// if we use auth, don't pass on Authoriation header
				req.Header.Del("Authorization")
			}
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "") // don't allow default value here
			}
		},
	}
}
