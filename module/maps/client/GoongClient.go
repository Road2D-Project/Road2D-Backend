package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"Road-To-Destination-BE/module/share"

	"golang.org/x/time/rate"
)

const (
	MaxIdleConnections    = 20
	IdleConnectionTimeout = 30 * time.Second
	RequestTimeout        = 10 * time.Second
	defaultDomain          = "https://rsapi.goong.io"
	freeRateLimit          = 5
)

type GoongClient struct {
	client  *http.Client
	limiter *rate.Limiter
}

var _ IMapClient = (*GoongClient)(nil)

func NewDefaultGoongClient() *GoongClient {
	transport := &http.Transport{
		MaxIdleConns:       MaxIdleConnections,
		IdleConnTimeout:    IdleConnectionTimeout,
		DisableCompression: true,
	}
	return &GoongClient{
		client:  &http.Client{Transport: transport, Timeout: RequestTimeout},
		limiter: rate.NewLimiter(rate.Limit(freeRateLimit), freeRateLimit),
	}
}

func (g *GoongClient) calcKey() string {
	return share.GetEnvStringDefault("GOONG_MAP_CALC_API_KEY", "")
}

func (g *GoongClient) baseURL() string {
	domain := strings.TrimRight(share.GetEnvStringDefault("GOONG_DOMAIN", defaultDomain), "/")
	domain = strings.TrimSuffix(domain, "/v2")
	if domain == "" {
		return defaultDomain
	}
	return domain
}

func (g *GoongClient) getJSON(ctx context.Context, path string, query url.Values, dest any) error {
	key := g.calcKey()
	if key == "" {
		return ErrMissingAPIKey
	}
	if err := g.limiter.Wait(ctx); err != nil {
		return err
	}

	query.Set("api_key", key)
	endpoint := g.baseURL() + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		_, _ = io.Copy(io.Discard, resp.Body)
		return ErrRateLimited
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("goong %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}

func goongFailed(status string) bool {
	return status != "" && status != "OK"
}
