package controller

import (
	"Road-To-Destination-BE/utils/enum"
	"errors"
	"net/http"

	"Road-To-Destination-BE/module/maps/client"
	"Road-To-Destination-BE/module/maps/model"
	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/maps/repository"
	"Road-To-Destination-BE/module/maps/service"
	"Road-To-Destination-BE/module/share"
	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/customValidator"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var _ share.PlaygroundRegistrar = (*MapController)(nil)

// Keep swagger types in this file so swag can resolve them.
var (
	_ = tripmodel.Leg{}
	_ = model.Trip{}
	_ = response.AutocompleteResponse{}
	_ = response.PlaceDetailResponse{}
	_ = response.GeocodeResponse{}
	_ = share.ErrorResponse{}
)

// MapController holds process-scoped infra only (HTTP client, leg cache).
// Use-case services are constructed inside each handler — not in main.
type MapController struct {
	goong *client.GoongClient
	legs  *repository.LocationLegMemoryStore
}

func NewMapController(redisClient *redis.Client) *MapController {
	return &MapController{
		goong: client.NewDefaultGoongClient(),
		legs:  repository.NewLocationLegMemoryStore(redisClient),
	}
}

func (ctrl *MapController) RegisterPlayground(router *gin.RouterGroup) {
	goong := router.Group("/goong")
	places := goong.Group("/places")
	{
		places.GET("/autocomplete", ctrl.HandleAutocomplete)
		places.GET("/detail", ctrl.HandleDetailPlace)
	}
	goong.GET("/directions", ctrl.HandleDirection)
	goong.GET("/trips", ctrl.HandleTrip)
	goong.GET("/geocode", ctrl.HandleGeocode)
}

// HandleAutocomplete godoc
// @Summary      Autocomplete places
// @Description  Goong Place Autocomplete v2. location biases the search; origin (lat,lng) sorts by proximity and fills distance_meters. If origin is omitted, location is reused. Set has_deprecated_administrative_unit=true to also get pre-merger names.
// @Tags         goong
// @Produce      json
// @Security     PlaygroundKey
// @Param        input                               query     string  true   "Search keyword"
// @Param        location                             query     string  false  "Bias as lat,lng"
// @Param        origin                               query     string  false  "Sort by distance from this lat,lng. Defaults to location"
// @Param        limit                                query     int     false  "Max predictions"
// @Param        radius                               query     int     false  "Search radius in km from location. Goong default 50"
// @Param        has_deprecated_administrative_unit     query     bool    false  "true = also return deprecated_description / deprecated_compound"
// @Success      200          {object}  response.AutocompleteResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /goong/places/autocomplete [get]
func (ctrl *MapController) HandleAutocomplete(c *gin.Context) {
	var req request.AutocompleteRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, customValidator.HandleValidationError(err))
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
// @Description  Goong Place Detail v2 by place_id. Default address is the new administrative unit.
// @Tags         goong
// @Produce      json
// @Security     PlaygroundKey
// @Param        place_id                             query     string  true   "Goong place_id"
// @Param        has_deprecated_administrative_unit    query     bool    false  "true = also return deprecated_description / deprecated_compound"
// @Success      200          {object}  response.PlaceDetailResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /goong/places/detail [get]
func (ctrl *MapController) HandleDetailPlace(c *gin.Context) {
	var req request.DetailPlaceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, customValidator.HandleValidationError(err))
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
// @Description  Goong Directions v2. Default vehicle is bike (two-wheeler / motorbike lanes). motorcycle and motorbike are aliased to bike.
// @Tags         goong
// @Produce      json
// @Security     PlaygroundKey
// @Param        origin        query     string  true   "Origin lat,lng"
// @Param        destination   query     string  true   "Destination lat,lng (semicolon-separated for extra stops)"
// @Param        vehicle       query     string  false  "car, bike, taxi, truck, hd. motorbike/motorcycle → bike"
// @Param        alternatives  query     bool    false  "Return alternatives; skips Leg cache"
// @Success      200          {object}  tripmodel.Leg
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /goong/directions [get]
func (ctrl *MapController) HandleDirection(c *gin.Context) {
	var req request.DirectionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, customValidator.HandleValidationError(err))
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

// HandleTrip godoc
// @Summary      Optimize multi-stop trip
// @Description  Goong Trip v2 is a single-vehicle TSP, not a branch graph. waypoints is a bag of lat,lng points (semicolon-separated), not an edge list. The waypoints array stays in input order; waypoint_index / visit_order is the visit sequence on that one tour. trips_index is copied from OSRM and is 0 unless Goong returns several tours — it does not mean a split at a shared vertex.
// @Tags         goong
// @Produce      json
// @Security     PlaygroundKey
// @Param        origin        query     string  false  "Start lat,lng. If omitted Goong picks a stop."
// @Param        destination   query     string  false  "End lat,lng. If omitted Goong picks a stop."
// @Param        waypoints     query     string  false  "Stops between origin and destination, lat,lng separated by ;"
// @Param        vehicle       query     string  false  "car, bike, taxi, truck, hd. Default car. motorbike/motorcycle → bike"
// @Param        roundtrip     query     bool    false  "Return to start. Default true"
// @Param        steps         query     bool    false  "Turn-by-turn per leg. Default true (Goong/OSRM default is false, which yields empty steps)"
// @Success      200          {object}  model.Trip
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /goong/trips [get]
func (ctrl *MapController) HandleTrip(c *gin.Context) {
	var req request.TripRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, customValidator.HandleValidationError(err))
		return
	}
	trips := service.NewTripService(ctrl.goong, ctrl.legs)
	out, err := trips.Optimize(c.Request.Context(), req)
	if err != nil {
		respondMapError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleGeocode godoc
// @Summary      Geocode address or coordinates
// @Description  Goong Geocode v2. Provide exactly one of address (forward), latlng (reverse), or place_id. Default results use the new administrative units. has_deprecated_administrative_unit=true adds pre-merger names. has_vnid=true adds deprecated_compound_id on reverse.
// @Tags         goong
// @Produce      json
// @Security     PlaygroundKey
// @Param        address                              query     string  false  "Forward: address to coordinates"
// @Param        latlng                               query     string  false  "Reverse: lat,lng to address"
// @Param        place_id                             query     string  false  "Lookup by Goong place_id"
// @Param        limit                                 query     int     false  "Max results (reverse)"
// @Param        has_deprecated_administrative_unit    query     bool    false  "true = also return deprecated_description / deprecated_compound"
// @Param        has_vnid                             query     bool    false  "true = also return deprecated_compound_id (VN admin codes)"
// @Success      200          {object}  response.GeocodeResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      429          {object}  share.ErrorResponse
// @Failure      502          {object}  share.ErrorResponse
// @Router       /goong/geocode [get]
func (ctrl *MapController) HandleGeocode(c *gin.Context) {
	var req request.GeocodeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, customValidator.HandleValidationError(err))
		return
	}
	geocode := service.NewGeocodeService(ctrl.goong)
	out, err := geocode.Lookup(c.Request.Context(), req)
	if err != nil {
		respondMapError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func respondMapError(c *gin.Context, err error) {
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, enum.ErrUnsupportedVehicle),
		errors.Is(err, service.ErrTooFewTripPoints),
		errors.Is(err, service.ErrRoundtripSameEnds),
		errors.Is(err, service.ErrGeocodeLookup),
		errors.Is(err, service.ErrInvalidLatLng):
		status = http.StatusBadRequest
	case errors.Is(err, client.ErrMissingAPIKey):
		status = http.StatusInternalServerError
	case errors.Is(err, client.ErrRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, client.ErrEmptyRoute), errors.Is(err, client.ErrEmptyTrip), errors.Is(err, client.ErrGoongStatus):
		status = http.StatusBadGateway
	}
	c.JSON(status, share.NewError(status, err.Error()))
}
