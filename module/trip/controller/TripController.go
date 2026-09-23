package controller

import (
	"Road-To-Destination-BE/middleware"
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

var (
	_ = model.Trip{}
	_ = model.TripMember{}
	_ = request.CreateTripRequest{}
	_ = request.UpdateTripRequest{}
	_ = response.TripResponse{}
	_ = response.TripListResponse{}
	_ = response.TripInviteLinkResponse{}
	_ = response.TripDetailResponse{}
	_ = share.ErrorResponse{}
)

type TripController struct {
	db          *gorm.DB
	redisClient *redis.Client
	validator   *validator.Validate
	auth        *middleware.AuthenticationMiddleware
}

func NewTripController(db *gorm.DB, redisClient *redis.Client, validator *validator.Validate, auth *middleware.AuthenticationMiddleware) *TripController {
	return &TripController{db: db, redisClient: redisClient, validator: validator, auth: auth}
}

func (ctrl *TripController) RegisterRoutes(router *gin.RouterGroup) {
	trips := router.Group("/trips", ctrl.auth.RequireAuth())
	{
		trips.POST("", ctrl.HandleCreateTrip())
		trips.GET("", ctrl.HandleListTrips())
		trips.POST("/join/:token", ctrl.HandleJoinTrip())
		trips.GET("/:tripId", ctrl.HandleGetTrip())
		trips.PUT("/:tripId/graph", ctrl.handleActiveTripRole(), ctrl.requireTripRole(enum.TripRoleLeader), ctrl.HandleSetTripGraph())
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
	}
}
