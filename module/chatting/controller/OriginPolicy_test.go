package controller

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestOriginPolicyAllowsOnlyExactOriginsOrSameOriginByDefault(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		configured []string
		origin     string
		want       bool
	}{
		{"native bearer client", nil, "", true},
		{"same origin", nil, "http://backend.test", true},
		{"foreign default", nil, "https://app.example.test", false},
		{"configured frontend", []string{"https://app.example.test"}, "https://app.example.test", true},
		{"prefix spoof", []string{"https://app.example.test"}, "https://app.example.test.evil.test", false},
		{"origin null", nil, "null", false},
		{"origin with path", nil, "http://backend.test/private", false},
		{"scheme mismatch", nil, "https://backend.test", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			upgrader, err := newUpgrader(scenario.configured)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("GET", "http://backend.test/v1/realtime/me", nil)
			if scenario.origin != "" {
				req.Header.Set("Origin", scenario.origin)
			}
			req.Header.Set("X-Forwarded-Host", "untrusted.test")
			if got := upgrader.CheckOrigin(req); got != scenario.want {
				t.Fatalf("allowed = %v", got)
			}
		})
	}
	for _, origin := range []string{"*", "https://*.example.test", "https://user:password@app.test", "https://app.test/path", "https://app.test?token=secret"} {
		if _, err := newUpgrader([]string{origin}); !errors.Is(err, ErrConfiguration) {
			t.Fatalf("invalid configuration accepted: %s", origin)
		}
	}
}
