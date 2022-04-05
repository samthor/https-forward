package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
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
	lock        sync.Mutex
	configFixed map[string]*hostConfig
	configMatch []func(string) *hostConfig
	transport   *http.Transport
}

func (ch *configHolder) For(host string) (*hostConfig, bool) {
	ch.lock.Lock()
	defer ch.lock.Unlock()

	out, ok := ch.configFixed[host]
	if ok {
		return out, true
	}

	for _, m := range ch.configMatch {
		config := m(host)
		if config != nil {
			ch.configFixed[host] = config
			return config, true
		}
	}

	return nil, false
}

func (ch *configHolder) Read(f string) error {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}

	var suffix string

	ch.lock.Lock()
	defer ch.lock.Unlock()

	ch.configFixed = make(map[string]*hostConfig)
	ch.configMatch = make([]func(string) *hostConfig, 0)

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
			if suffix == "." {
				suffix = ""
				log.Printf("config- reset suffix")
			} else {
				log.Printf("config- found suffix: %s", suffix)
			}
			continue
		}

		// otherwise we expect "blah localhost:8080 opt_user:and_pass"
		if len(f) > 3 || len(f) == 0 {
			continue
		}
		qualified := string(f[0]) + suffix
		hc := &hostConfig{}

		if len(f) > 2 {
			parts := bytes.SplitN(f[2], []byte{byte(':')}, 2)
			hc.username = string(parts[0])
			if len(parts) == 2 {
				hc.password = string(parts[1])
			}
			hc.auth = true
		}

		if len(f) > 1 {
			hc.target = string(f[1])
			hc.proxy = buildProxy(hc.target, hc.auth, ch.transport)
		}

		if !isValidDomain(qualified) {
			log.Printf("config- skipping invalid domain: %s", qualified)
			continue
		}

		match := buildMatch(qualified)
		matchHostConfig := func(q string) *hostConfig {
			if match(q) {
				return hc
			}
			return nil
		}
		ch.configMatch = append(ch.configMatch, matchHostConfig)
		log.Printf("config- matched: %s => %s", qualified, hc.target)
	}
	log.Printf("config- DONE, %d valid entries", len(ch.configMatch))

	return nil
}

// Checks domain for validity. Allows * and ? for globbing.
func isValidDomain(q string) bool {
	index := strings.IndexFunc(q, func(r rune) bool {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '*' || r == '?'
		return !ok
	})
	return index == -1
}

func buildMatch(q string) func(string) bool {
	if !strings.Contains(q, "*") {
		return func(check string) bool {
			return check == q
		}
	}

	re := q
	re = strings.ReplaceAll(re, ".", "\\.")
	re = strings.ReplaceAll(re, "?", "[a-z0-9]")   // this means a single char
	re = strings.ReplaceAll(re, "*", "[a-z0-9]*?") // this means any number of chars
	re = fmt.Sprintf("^%s$", re)

	log.Printf("compiling regexp for domain=%v re=%v", q, re)

	r := regexp.MustCompile(re)

	return func(check string) bool {
		return r.MatchString(check)
	}
}

func buildProxy(target string, auth bool, transport http.RoundTripper) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Transport: transport,
		Director: func(req *http.Request) {
			// nb. go1.18 at least only adds "X-Forwarded-For", nothing else
			req.Header.Set("X-Forwarded-Host", req.Host)

			forwardParts := []string{
				"proto=https",
				fmt.Sprintf("host=%s", req.Host),
			}
			if req.RemoteAddr != "" {
				isV6 := !strings.Contains(req.RemoteAddr, ".")
				if isV6 {
					forwardParts = append(forwardParts, fmt.Sprintf("for=\"%s\"", req.RemoteAddr))
				} else {
					forwardParts = append(forwardParts, fmt.Sprintf("for=%s", req.RemoteAddr))
				}
			}
			req.Header.Add("Forwarded", strings.Join(forwardParts, ";"))

			if auth {
				// if we use auth, don't pass on Authoriation header
				req.Header.Del("Authorization")
			}
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "") // don't allow default value here
			}

			req.Host = target
			req.URL.Scheme = "http"
			req.URL.Host = target
		},
	}
}
