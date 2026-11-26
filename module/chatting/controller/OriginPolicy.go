package controller

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func newUpgrader(origins []string) (websocket.Upgrader, error) {
	allowed := make(map[string]bool)
	for _, origin := range origins {
		if strings.TrimSpace(origin) == "" {
			continue
		}
		canonical, ok := canonicalOrigin(strings.TrimSpace(origin))
		if !ok {
			return websocket.Upgrader{}, ErrConfiguration
		}
		allowed[canonical] = true
	}
	return websocket.Upgrader{
		ReadBufferSize: 1024, WriteBufferSize: 1024, HandshakeTimeout: 5 * time.Second,
		CheckOrigin: func(r *http.Request) bool {
			origins := r.Header.Values("Origin")
			if len(origins) == 0 {
				return true // Native clients still require a Bearer access token.
			}
			if len(origins) != 1 {
				return false
			}
			origin, ok := canonicalOrigin(origins[0])
			if !ok {
				return false
			}
			if len(allowed) > 0 {
				return allowed[origin]
			}
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			return origin == scheme+"://"+strings.ToLower(r.Host)
		},
	}, nil
}

func canonicalOrigin(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || strings.Contains(parsed.Host, "*") || parsed.User != nil || parsed.Opaque != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	return parsed.Scheme + "://" + strings.ToLower(parsed.Host), true
}
