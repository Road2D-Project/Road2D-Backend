package client

// Temporary scratch client for SerpApi Google Maps. Safe to delete later.
// Docs: https://serpapi.com/google-maps-api
//
// Env: SERP_MAP_API
// Goong place_id ≠ Google place_id. SerpApi wants Google ids (ChIJ...).

import (
	"fmt"
	"net/http"
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
	api serpapi.SerpApiClient
	key string
}

func NewSerpClient() *SerpClient {
	key := share.GetEnvStringDefault("SERP_MAP_API", "")
	setting := serpapi.NewSerpApiClientSetting(key)
	setting.Engine = "google_maps"
	return &SerpClient{api: serpapi.NewClient(setting), key: key}
}

func (s *SerpClient) RegisterRoutes(router *gin.RouterGroup) {
	g := router.Group("/serp-api")
	g.GET("/search", s.handleSearch)
	g.GET("/ll", s.handleByCoord)
	g.GET("/place", s.handleByPlaceQuery)
	g.GET("/:placeId", s.handleByPlacePath)
}

func (s *SerpClient) handleSearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "q is required"))
		return
	}
	params := s.mapsSearchParams()
	params["type"] = "search"
	params["q"] = q
	if ll := llFromQuery(c); ll != "" {
		params["ll"] = ll
	}
	s.respond(c, params)
}

func (s *SerpClient) handleByCoord(c *gin.Context) {
	ll := llFromQuery(c)
	if ll == "" {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "lat and lng are required"))
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		q = "place"
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
	if placeID == "" {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "place_id is required"))
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
	out, err := s.api.Search(params)
	if err != nil {
		c.JSON(http.StatusBadGateway, share.NewError(http.StatusBadGateway, err.Error()))
		return
	}
	c.JSON(http.StatusOK, out)
}

func llFromQuery(c *gin.Context) string {
	lat := strings.TrimSpace(c.Query("lat"))
	lng := strings.TrimSpace(c.Query("lng"))
	if lat == "" || lng == "" {
		if raw := strings.TrimSpace(c.Query("ll")); raw != "" {
			if strings.HasPrefix(raw, "@") {
				return raw
			}
			return "@" + raw
		}
		return ""
	}
	zoom := strings.TrimSpace(c.Query("zoom"))
	if zoom == "" {
		zoom = serpDefaultZoom
	}
	if !strings.HasSuffix(zoom, "z") && !strings.HasSuffix(zoom, "m") {
		zoom += "z"
	}
	return fmt.Sprintf("@%s,%s,%s", lat, lng, zoom)
}
