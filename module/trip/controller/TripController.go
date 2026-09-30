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
	_ = request.ForkTripRequest{}
	_ = request.UpdateTripMemberRequest{}
	_ = request.ComputeBranchRequest{}
	_ = request.PreviewLocationsRequest{}
	_ = request.ComputeTravelGraphRequest{}
	_ = request.UpdateBranchStopRequest{}
	_ = request.AddDraftDestinationRequest{}
	_ = response.DraftDestinationsResponse{}
	_ = response.TripResponse{}
	_ = response.TripListResponse{}
	_ = response.TripMemberResponse{}
	_ = response.TripMemberListResponse{}
	_ = response.TripInvitationListResponse{}
	_ = response.TripJoinRequestListResponse{}
	_ = response.TripInviteLinkResponse{}
	_ = response.TripDetailResponse{}
	_ = response.ComputeBranchResponse{}
	_ = response.PreviewLocationsResponse{}
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
	authed := ctrl.auth.RequireAuth()
	trips := router.Group("/trips", authed)
	ctrl.registerTripRoutes(trips)
	ctrl.registerMemberRoutes(trips)
	ctrl.registerGraphRoutes(trips)
	ctrl.registerPlanningRoutes(router.Group("/planning", authed))
}

func (ctrl *TripController) registerTripRoutes(trips *gin.RouterGroup) {
	trips.POST("", ctrl.HandleCreateTrip())
	trips.GET("", ctrl.HandleListTrips())
	trips.GET("/public", ctrl.HandleListPublicTrips())
	trips.GET("/:tripId", ctrl.HandleGetTrip())
	trips.POST("/:tripId/fork", ctrl.handleActiveTripRole(), ctrl.HandleForkTrip())
	trips.PATCH("/:tripId", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader, enum.TripRoleAdmin), ctrl.HandleUpdateTrip())
	trips.DELETE("/:tripId", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleDeleteTrip())
}

func (ctrl *TripController) registerMemberRoutes(trips *gin.RouterGroup) {
	trips.GET("/invitations", ctrl.HandleListTripInvitations())
	trips.POST("/join/:token", ctrl.HandleJoinTrip())
	trips.GET("/:tripId/members", ctrl.handleActiveTripRole(), ctrl.HandleListTripMembers())
	trips.PATCH("/:tripId/members/:userId", ctrl.handleActiveTripRole(), ctrl.HandleUpdateTripMember())
	trips.DELETE("/:tripId/members/:userId", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader, enum.TripRoleAdmin), ctrl.HandleKickTripMember())
	trips.POST("/:tripId/invite/:userId", ctrl.handleActiveTripRole(), ctrl.HandleInviteTripMember())
	trips.POST("/:tripId/invitations", ctrl.HandleRespondTripInvitation())
	trips.GET("/:tripId/join-requests", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader, enum.TripRoleAdmin), ctrl.HandleListTripJoinRequests())
	trips.POST("/:tripId/join-requests/:userId", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader, enum.TripRoleAdmin), ctrl.HandleTripJoinRequest())
	trips.POST("/:tripId/invite-link", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader, enum.TripRoleAdmin), ctrl.HandleCreateInviteLink())
	trips.POST("/:tripId/leave", ctrl.handleActiveTripRole(), ctrl.HandleLeaveTrip())
}

func (ctrl *TripController) registerGraphRoutes(trips *gin.RouterGroup) {
	trips.GET("/:tripId/graph", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph"), ctrl.cacheResponse.CacheAround(), ctrl.HandleGetTripGraph())
	trips.PUT("/:tripId/graph", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleSetTripGraph())
	trips.GET("/:tripId/draft", ctrl.handleActiveTripRole(), ctrl.CachedKeys("draft"), ctrl.cacheResponse.CacheAround(), ctrl.HandleListDraftDestinations())
	trips.POST("/:tripId/draft", ctrl.handleActiveTripRole(), ctrl.CachedKeys("draft"), ctrl.cacheResponse.MarkChanged(), ctrl.HandleAddDraftDestination())
	trips.POST("/:tripId/compute", ctrl.handleActiveTripRole(), ctrl.HandleComputeTrip())
	trips.GET("/:tripId/travels", ctrl.handleActiveTripRole(), ctrl.CachedKeys("travels"), ctrl.cacheResponse.CacheAround(), ctrl.HandleGetStoredTravels())
	trips.POST("/:tripId/travels", ctrl.handleActiveTripRole(), ctrl.CachedKeys("travels"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleComputeStoredTrip())
	trips.POST("/:tripId/travels/graph", ctrl.handleActiveTripRole(), ctrl.CachedKeys("travels", "graph"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleComputeTravelGraph())
	trips.PATCH("/:tripId/branches/:branchId/stops/:destinationId", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph", "travels"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleUpdateBranchStop())
	trips.DELETE("/:tripId/branches/:branchId/stops/:destinationId", ctrl.handleActiveTripRole(), ctrl.CachedKeys("graph", "travels"), ctrl.cacheResponse.MarkChanged(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleDeleteBranchStop())
}

func (ctrl *TripController) registerPlanningRoutes(planning *gin.RouterGroup) {
	planning.POST("/preview", ctrl.HandlePreviewLocations())
	planning.POST("/fork/:locationId", ctrl.HandleForkLocation())
	planning.GET("/location/:locationId", ctrl.HandleGetLocation())
	planning.GET("/destination/:destinationId", ctrl.HandleGetDestination())
	planning.PUT("/destination/:destinationId", ctrl.HandleUpdateDestination())
	planning.DELETE("/destination/:destinationId", ctrl.cacheResponse.MarkChanged(), ctrl.HandleDeleteDestination())
}
