package controller

import (
	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	groupRepo "Road-To-Destination-BE/module/group/repository"
	mapsclient "Road-To-Destination-BE/module/maps/client"
	mapsrepo "Road-To-Destination-BE/module/maps/repository"
	mapsservice "Road-To-Destination-BE/module/maps/service"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/module/trip/service"
	"Road-To-Destination-BE/utils/customValidator"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (ctrl *TripController) tripService() *service.TripService {
	return service.NewTripService(
		repository.NewTripRepository(ctrl.db),
		groupRepo.NewGroupRepository(ctrl.db),
		groupRepo.NewGroupMemberRepository(ctrl.db),
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberStore(ctrl.redisClient),
	)
}
func (ctrl *TripController) tripBranch() *service.TripBranchService {
	return service.NewTripBranchService(
		repository.NewTripBranchRepository(ctrl.db),
		repository.NewTripRepository(ctrl.db),
	)
}
func (ctrl *TripController) leaveTripService() *service.LeaveTripService {
	return service.NewLeaveTripService(
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberStore(ctrl.redisClient),
	)
}

func (ctrl *TripController) joinTripService() *service.JoinTripService {
	return service.NewJoinTripService(
		repository.NewTripRepository(ctrl.db),
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberStore(ctrl.redisClient),
	)
}

func (ctrl *TripController) computeTrip() *service.ComputeTripService {
	return service.NewComputeTripService(
		mapsservice.NewDirectionService(ctrl.goong, ctrl.previewLegs()),
		repository.NewTravelRepository(ctrl.db),
		repository.NewDestinationRepository(ctrl.db),
		repository.NewLocationRepository(ctrl.db),
	)
}

// previewLegs is Redis only. A miss falls through to Goong inside DirectionService
// and comes back here with a 15 minute TTL. Postgres is not on this path.
func (ctrl *TripController) previewLegs() mapsrepo.LegStore {
	if ctrl.redisClient == nil {
		return noopLegStore{}
	}
	return shortTTLLegStore{inner: mapsrepo.NewLocationLegMemoryStore(ctrl.redisClient)}
}

func (ctrl *TripController) placeService() *service.PlaceService {
	return service.NewLocationService(
		repository.NewDestinationRepository(ctrl.db),
		repository.NewLocationRepository(ctrl.db),
	)
}

func mapTripError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrTripNotFound),
		errors.Is(err, repository.ErrGroupNotFound),
		errors.Is(err, repository.ErrInviteNotFound),
		errors.Is(err, repository.ErrDestinationNotFound),
		errors.Is(err, repository.ErrLocationNotFound):
		jsonError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrAlreadyTripMember),
		errors.Is(err, repository.ErrNoSuccessorToTransfer),
		errors.Is(err, repository.ErrDestinationNotEditing):
		jsonError(c, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrUserNotTripMember),
		errors.Is(err, repository.ErrUserNotGroupMember):
		jsonError(c, http.StatusForbidden, err.Error())
	case errors.Is(err, repository.ErrNoTripUpdate):
		jsonError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrInternalServerError),
		errors.Is(err, mapsclient.ErrMissingAPIKey):
		jsonError(c, http.StatusInternalServerError, err.Error())
	case errors.Is(err, mapsclient.ErrRateLimited):
		jsonError(c, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, mapsclient.ErrEmptyRoute),
		errors.Is(err, mapsclient.ErrEmptyTrip),
		errors.Is(err, mapsclient.ErrGoongStatus):
		jsonError(c, http.StatusBadGateway, err.Error())
	default:
		jsonError(c, http.StatusBadRequest, err.Error())
	}
}

func parseTripID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "tripId")
}

func parseLocationID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "locationId")
}

func parseDestinationID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "destinationId")
}

func parsePathID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

func currentUserOrAbort(c *gin.Context) *authModel.User {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Unauthorized"))
		return nil
	}
	return user
}

func jsonError(c *gin.Context, status int, message string) {
	c.JSON(status, share.NewError(status, message))
}

func jsonBindError(c *gin.Context, err error) {
	body := customValidator.HandleValidationError(err)
	c.JSON(body.Status, body)
}
