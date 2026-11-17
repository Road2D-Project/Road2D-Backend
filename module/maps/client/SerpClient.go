package client

// Temporary scratch client for SerpApi Google Maps. Safe to delete later.
// Docs: https://serpapi.com/google-maps-api
//
// Env: SERP_MAP_API
// Goong place_id ≠ Google place_id. SerpApi wants Google ids (ChIJ...).

import (
	"Road-To-Destination-BE/middleware"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
	"github.com/serpapi/serpapi-golang"
)

const (
	// Goong place coords are at 18z. Serp/Google recompute viewport when zoom
	// changes, so other values still work — keep 18z as default so a Goong
	// lat,lng and a Serp ll=@lat,lng,18z describe the same pin.
	serpDefaultZoom = "18z"
	serpDefaultHL   = "vi"
	serpDefaultGL   = "vn"
)

type SerpClient struct {
	api         serpapi.SerpApiClient
	key         string
	rateLimiter middleware.RateLimiter
	quota       *ProviderQuota
}

func NewSerpClient(limiter middleware.RateLimiter, quota *ProviderQuota) (*SerpClient, error) {
	if limiter == nil || quota == nil {
		return nil, errors.New("SerpAPI rate limiter and provider quota are required")
	}
	key := share.GetEnvStringDefault("SERP_MAP_API", "")
	setting := serpapi.NewSerpApiClientSetting(key)
	setting.Engine = "google_maps"
	setting.Timeout = RequestTimeout
	return &SerpClient{api: serpapi.NewClient(setting), key: key, rateLimiter: limiter, quota: quota}, nil
}

func (s *SerpClient) RegisterRoutes(router *gin.RouterGroup) {
	g := router.Group("/serp-api", s.rateLimiter.RequireIPLimiter())
	g.GET("/search", s.handleSearch)
	g.GET("/ll", s.handleByCoord)
	g.GET("/place", s.handleByPlaceQuery)
	g.GET("/:placeId", s.handleByPlacePath)
}

func (s *SerpClient) handleSearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" || len(q) > 512 {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "q must contain 1 to 512 bytes"))
		return
	}
	params := s.mapsSearchParams()
	params["type"] = "search"
	params["q"] = q
	ll, err := llFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, err.Error()))
		return
	}
	if ll != "" {
		params["ll"] = ll
	}
	s.respond(c, params)
}

func (s *SerpClient) handleByCoord(c *gin.Context) {
	ll, err := llFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, err.Error()))
		return
	}
	if ll == "" {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "lat and lng are required"))
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		q = "place"
	}
	if len(q) > 512 {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "q must contain at most 512 bytes"))
		return
	}
	params := s.mapsSearchParams()
	params["type"] = "search"
	params["q"] = q
	params["ll"] = ll
	s.respond(c, params)
}

func (s *SerpClient) handleByPlaceQuery(c *gin.Context) {
	placeID := strings.TrimSpace(c.Query("place_id"))
	if placeID == "" {
		placeID = strings.TrimSpace(c.Query("placeId"))
	}
	s.place(c, placeID)
}

func (s *SerpClient) handleByPlacePath(c *gin.Context) {
	s.place(c, strings.TrimSpace(c.Param("placeId")))
}

func (s *SerpClient) place(c *gin.Context, placeID string) {
	if placeID == "" || len(placeID) > 512 {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "place_id must contain 1 to 512 bytes"))
		return
	}
	params := s.mapsSearchParams()
	params["place_id"] = placeID
	s.respond(c, params)
}

func (s *SerpClient) mapsSearchParams() map[string]string {
	return map[string]string{
		"engine": "google_maps",
		"hl":     serpDefaultHL,
		"gl":     serpDefaultGL,
	}
}

func (s *SerpClient) respond(c *gin.Context, params map[string]string) {
	if s.key == "" {
		c.JSON(http.StatusServiceUnavailable, share.NewError(http.StatusServiceUnavailable, "SERP_MAP_API is not set"))
		return
	}
	if err := s.quota.Allow(c.Request.Context()); err != nil {
		status := http.StatusTooManyRequests
		if errors.Is(err, ErrQuotaUnavailable) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, share.NewError(status, err.Error()))
		return
	}
	out, err := s.api.Search(params)
	if err != nil {
		c.JSON(http.StatusBadGateway, share.NewError(http.StatusBadGateway, err.Error()))
		return
	}
	c.JSON(http.StatusOK, out)
}

func llFromQuery(c *gin.Context) (string, error) {
	lat := strings.TrimSpace(c.Query("lat"))
	lng := strings.TrimSpace(c.Query("lng"))
	zoom := strings.TrimSpace(c.Query("zoom"))
	if lat == "" && lng == "" {
		raw := strings.TrimSpace(c.Query("ll"))
		if raw == "" {
			return "", nil
		}
		parts := strings.Split(strings.TrimPrefix(raw, "@"), ",")
		if len(parts) < 2 || len(parts) > 3 {
			return "", errors.New("ll must contain latitude, longitude and an optional zoom")
		}
		lat, lng = parts[0], parts[1]
		if len(parts) == 3 {
			zoom = parts[2]
		}
	}
	latitude, latErr := strconv.ParseFloat(lat, 64)
	longitude, lngErr := strconv.ParseFloat(lng, 64)
	if latErr != nil || lngErr != nil || !(math.Abs(latitude) <= 90 && math.Abs(longitude) <= 180) {
		return "", errors.New("lat and lng must be valid geographic coordinates")
	}
	if zoom == "" {
		zoom = serpDefaultZoom
	}
	if !strings.HasSuffix(zoom, "z") && !strings.HasSuffix(zoom, "m") {
		zoom += "z"
	}
	value, err := strconv.ParseFloat(zoom[:len(zoom)-1], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return "", errors.New("zoom must be a finite non-negative number with a z or m suffix")
	}
	return fmt.Sprintf("@%s,%s,%s", lat, lng, zoom), nil
}
