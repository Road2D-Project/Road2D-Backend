package controller

import (
	"errors"
	"net/http"

	"Road-To-Destination-BE/module/maps/client"
	"Road-To-Destination-BE/module/maps/model"
	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/maps/repository"
	"Road-To-Destination-BE/module/maps/service"
	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
)

// Keep swagger types in this file so swag can resolve them.
var (
	_ = model.LocationLeg{}
	_ = response.AutocompleteResponse{}
	_ = response.PlaceDetailResponse{}
	_ = share.ErrorResponse{}
)

// MapController holds process-scoped infra only (HTTP client, leg cache).
// Use-case services are constructed inside each handler — not in main.
type MapController struct {
	goong *client.GoongClient
	legs  *repository.LocationLegMemoryStore
}

func NewMapController() *MapController {
	return &MapController{
		goong: client.NewDefaultGoongClient(),
		legs:  repository.NewLocationLegMemoryStore(),
	}
}

func (ctrl *MapController) RegisterRoutes(router *gin.RouterGroup) {
	places := router.Group("/places")
	{
		places.GET("/autocomplete", ctrl.HandleAutocomplete)
		places.GET("/detail", ctrl.HandleDetailPlace)
	}
	router.GET("/directions", ctrl.HandleDirection)
}

// HandleAutocomplete godoc
// @Summary      Autocomplete places
// @Description  Goong Place Autocomplete. Pass the same sessiontoken to /places/detail to bill as one session.
// @Tags         maps
// @Produce      json
// @Param        input         query     string  true   "Search keyword"
// @Param        location      query     string  false  "Bias as lat,lng"
// @Param        sessiontoken  query     string  false  "UUID v4 autocomplete session"
// @Param        limit         query     int     false  "Max predictions"
// @Success      200          {object}  response.AutocompleteResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /places/autocomplete [get]
func (ctrl *MapController) HandleAutocomplete(c *gin.Context) {
	var req request.AutocompleteRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
		return
	}
	places := service.NewPlaceService(ctrl.goong)
	out, err := places.Autocomplete(c.Request.Context(), req)
	if err != nil {
		respondMapError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleDetailPlace godoc
// @Summary      Place detail
// @Description  Goong Place Detail by place_id. Reuse sessiontoken from autocomplete.
// @Tags         maps
// @Produce      json
// @Param        place_id      query     string  true   "Goong place_id"
// @Param        sessiontoken  query     string  false  "UUID v4 autocomplete session"
// @Success      200          {object}  response.PlaceDetailResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /places/detail [get]
func (ctrl *MapController) HandleDetailPlace(c *gin.Context) {
	var req request.DetailPlaceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
		return
	}
	places := service.NewPlaceService(ctrl.goong)
	out, err := places.GetDetailPlace(c.Request.Context(), req)
	if err != nil {
		respondMapError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleDirection godoc
// @Summary      Route A to B
// @Description  Goong Directions v2. Cached as LocationLeg unless alternatives=true. Default vehicle is motorcycle.
// @Tags         maps
// @Produce      json
// @Param        origin        query     string  true   "Origin lat,lng"
// @Param        destination   query     string  true   "Destination lat,lng (semicolon-separated for extra stops)"
// @Param        vehicle       query     string  false  "car, bike, motorcycle, taxi, truck, hd"
// @Param        alternatives  query     bool    false  "Return alternatives; skips LocationLeg cache"
// @Success      200          {object}  model.LocationLeg
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /directions [get]
func (ctrl *MapController) HandleDirection(c *gin.Context) {
	var req request.DirectionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
		return
	}
	directions := service.NewDirectionService(ctrl.goong, ctrl.legs)
	out, err := directions.Route(c.Request.Context(), req)
	if err != nil {
		respondMapError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func respondMapError(c *gin.Context, err error) {
	body := share.ErrorResponse{Error: err.Error()}
	switch {
	case errors.Is(err, client.ErrMissingAPIKey):
		c.JSON(http.StatusInternalServerError, body)
	case errors.Is(err, client.ErrRateLimited):
		c.JSON(http.StatusTooManyRequests, body)
	case errors.Is(err, client.ErrEmptyRoute), errors.Is(err, client.ErrGoongStatus):
		c.JSON(http.StatusBadGateway, body)
	default:
		c.JSON(http.StatusBadGateway, body)
	}
}
