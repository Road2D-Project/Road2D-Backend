package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Road-To-Destination-BE/middleware"

	"github.com/gin-gonic/gin"
)

type serpTestTransport struct {
	calls int
}

func (s *serpTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.calls++
	return &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
		Body: io.NopCloser(strings.NewReader(`{"search_metadata":{"status":"Success"}}`)),
	}, nil
}

func TestSerpPlaygroundRequiresKeyAndSharesProviderBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GOONG_PLAYGROUND_KEY", "playground-secret")
	t.Setenv("SERP_MAP_API", "test-key")
	quota, _ := testProviderQuota(t, "serp", 1)
	limiter, err := middleware.NewTokenBucket(middleware.DefaultTokenBucketConfig(), quota.client)
	if err != nil {
		t.Fatal(err)
	}
	serp, err := NewSerpClient(limiter, quota)
	if err != nil {
		t.Fatal(err)
	}
	transport := &serpTestTransport{}
	serp.api.HttpSearch.Transport = transport
	router := gin.New()
	playground := router.Group("/v1", middleware.RequireHeaderPlaygroundKey())
	serp.RegisterRoutes(playground)
	for _, test := range []struct {
		path   string
		key    string
		status int
	}{
		{"/v1/serp-api/search?q=park", "", http.StatusUnauthorized},
		{"/v1/serp-api/search?q=park", "wrong-key", http.StatusUnauthorized},
		{"/v1/serp-api/search?q=" + strings.Repeat("x", 513), "playground-secret", http.StatusBadRequest},
		{"/v1/serp-api/ll?lat=NaN&lng=105", "playground-secret", http.StatusBadRequest},
		{"/v1/serp-api/ll?lat=91&lng=105", "playground-secret", http.StatusBadRequest},
		{"/v1/serp-api/search?q=park&ll=invalid", "playground-secret", http.StatusBadRequest},
		{"/v1/serp-api/search?q=park", "playground-secret", http.StatusOK},
		{"/v1/serp-api/ll?lat=21&lng=105", "playground-secret", http.StatusTooManyRequests},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set(middleware.PlaygroundKeyHeader, test.key)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("%s returned %d, want %d: %s", test.path, recorder.Code, test.status, recorder.Body.String())
		}
	}
	if transport.calls != 1 {
		t.Fatalf("SerpAPI received %d requests, want 1", transport.calls)
	}
}

func TestSerpMapCenterNormalizesSupportedInputs(t *testing.T) {
	for _, test := range []struct {
		query string
		want  string
	}{
		{"", ""},
		{"lat=21&lng=105", "@21,105,18z"},
		{"lat=21&lng=105&zoom=12", "@21,105,12z"},
		{"ll=21,105,200m", "@21,105,200m"},
		{"ll=@21,105,18z", "@21,105,18z"},
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+test.query, nil)
		got, err := llFromQuery(ctx)
		if err != nil || got != test.want {
			t.Fatalf("query %q returned %q, %v, want %q", test.query, got, err, test.want)
		}
	}
}

func TestSerpClientRejectsMissingLimiterOrQuota(t *testing.T) {
	quota, _ := testProviderQuota(t, "serp", 1)
	limiter, err := middleware.NewTokenBucket(middleware.DefaultTokenBucketConfig(), quota.client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSerpClient(nil, quota); err == nil {
		t.Fatal("missing rate limiter was accepted")
	}
	if _, err := NewSerpClient(limiter, nil); err == nil {
		t.Fatal("missing provider quota was accepted")
	}
}
