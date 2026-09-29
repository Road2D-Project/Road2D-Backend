package controller

import (
	"Road-To-Destination-BE/middleware"
	mapsclient "Road-To-Destination-BE/module/maps/client"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/utils/enum"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var _ share.RouterRegistrar = (*TripController)(nil)
var _ share.ResponseCacheRegistrar = (*TripController)(nil)

var (
	_ = model.Trip{}
	_ = model.TripMember{}
	_ = request.CreateTripRequest{}
	_ = request.UpdateTripRequest{}
	_ = request.ComputeBranchRequest{}
	_ = request.ComputeTravelGraphRequest{}
	_ = request.UpdateBranchStopRequest{}
	_ = request.AddDraftDestinationRequest{}
	_ = response.DraftDestinationsResponse{}
	_ = response.TripResponse{}
	_ = response.TripListResponse{}
	_ = response.TripInviteLinkResponse{}
	_ = response.TripDetailResponse{}
	_ = response.ComputeBranchResponse{}
	_ = response.ComputeTripResponse{}
	_ = share.ErrorResponse{}
)

type TripController struct {
	db            *gorm.DB
	redisClient   *redis.Client
	validator     *validator.Validate
	auth          *middleware.AuthenticationMiddleware
	goong         *mapsclient.GoongClient
	cacheResponse *middleware.ResponseCache
}

func (ctrl *TripController) CachedKeys(names ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tripId := c.Param("tripId")
		if tripId == "" || len(names) == 0 {
			c.Next()
			return
		}
		keys := make([]string, len(names))
		for i, name := range names {
			keys[i] = "resp:" + name + ":" + tripId
		}
		c.Set(middleware.MainKeyController, keys)
		c.Next()
	}
}

func NewTripController(db *gorm.DB, redisClient *redis.Client, validator *validator.Validate, auth *middleware.AuthenticationMiddleware, cacheResponse *middleware.ResponseCache) *TripController {
	return &TripController{
		db:            db,
		redisClient:   redisClient,
		validator:     validator,
		auth:          auth,
		goong:         mapsclient.NewDefaultGoongClient(),
		cacheResponse: cacheResponse,
	}
}

func (ctrl *TripController) RegisterRoutes(router *gin.RouterGroup) {
	trips := router.Group("/trips", ctrl.auth.RequireAuth())
	{
		trips.POST("", ctrl.HandleCreateTrip())
		trips.GET("", ctrl.HandleListTrips())
		trips.POST("/join/:token", ctrl.HandleJoinTrip())
		trips.GET("/:tripId", ctrl.HandleGetTrip())
		trips.GET("/:tripId/graph", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph"), ctrl.cacheResponse.CacheAround(), ctrl.HandleGetTripGraph())
		trips.GET("/:tripId/draft", ctrl.handleActiveTripRole(), ctrl.CachedKeys("draft"), ctrl.cacheResponse.CacheAround(), ctrl.HandleListDraftDestinations())
		trips.POST("/:tripId/draft", ctrl.handleActiveTripRole(), ctrl.CachedKeys("draft"), ctrl.cacheResponse.MarkChanged(), ctrl.HandleAddDraftDestination())
		trips.PUT("/:tripId/graph", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleSetTripGraph())
		trips.POST("/:tripId/compute", ctrl.handleActiveTripRole(), ctrl.HandleComputeTrip())
		trips.GET("/:tripId/travels", ctrl.handleActiveTripRole(), ctrl.CachedKeys("travels"), ctrl.cacheResponse.CacheAround(), ctrl.HandleGetStoredTravels())
		trips.POST("/:tripId/travels", ctrl.handleActiveTripRole(), ctrl.CachedKeys("travels"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleComputeStoredTrip())
		trips.POST("/:tripId/travels/graph", ctrl.handleActiveTripRole(), ctrl.CachedKeys("travels", "graph"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleComputeTravelGraph())
		trips.PATCH("/:tripId/branches/:branchId/stops/:destinationId", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph", "travels"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleUpdateBranchStop())
		trips.DELETE("/:tripId/branches/:branchId/stops/:destinationId", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph", "travels"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleDeleteBranchStop())
		trips.PATCH("/:tripId", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleUpdateTrip())
		trips.DELETE("/:tripId", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleDeleteTrip())
		trips.POST("/:tripId/invite-link", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleCreateInviteLink())
		trips.POST("/:tripId/leave", ctrl.handleActiveTripRole(), ctrl.HandleLeaveTrip())
	}
	// liên quan tới place{location,destination}
	planing := router.Group("/planing", ctrl.auth.RequireAuth())
	{
		planing.POST("/fork/:locationId", ctrl.HandleForkLocation())
		planing.GET("/location/:locationId", ctrl.HandleGetLocation())
		planing.GET("/destination/:destinationId", ctrl.HandleGetDestination())
		planing.PUT("/destination/:destinationId", ctrl.HandleUpdateDestination())
		planing.DELETE("/destination/:destinationId", ctrl.cacheResponse.MarkChanged(), ctrl.HandleDeleteDestination())
	}
}
