package controller

import (
	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	authRepo "Road-To-Destination-BE/module/authentication/repository"
	mapsclient "Road-To-Destination-BE/module/maps/client"
	mapsrepo "Road-To-Destination-BE/module/maps/repository"
	mapsservice "Road-To-Destination-BE/module/maps/service"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/module/trip/service"
	"Road-To-Destination-BE/utils/customValidator"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	membershipActionAccept = "accept"
	membershipActionReject = "reject"
)

func (ctrl *TripController) tripService() *service.TripService {
	return service.NewTripService(
		repository.NewTripRepository(ctrl.db),
		authRepo.NewUserRepository(ctrl.db),
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberStore(ctrl.redisClient),
		repository.NewLocationRepository(ctrl.db),
	)
}

func (ctrl *TripController) tripMemberService() *service.TripMemberService {
	return service.NewTripMemberService(
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberStore(ctrl.redisClient),
	)
}

func (ctrl *TripController) inviteTripMemberService() *service.InviteTripMemberService {
	return service.NewInviteTripMemberService(
		repository.NewTripRepository(ctrl.db),
		repository.NewTripMemberRepository(ctrl.db),
	)
}

func (ctrl *TripController) tripInvitationService() *service.TripInvitationService {
	return service.NewTripInvitationService(
		repository.NewTripRepository(ctrl.db),
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberStore(ctrl.redisClient),
	)
}

func (ctrl *TripController) tripJoinRequestService() *service.TripJoinRequestService {
	return service.NewTripJoinRequestService(
		repository.NewTripRepository(ctrl.db),
		repository.NewTripMemberRepository(ctrl.db),
		repository.NewTripMemberStore(ctrl.redisClient),
	)
}

func (ctrl *TripController) kickTripMemberService() *service.KickTripMemberService {
	return service.NewKickTripMemberService(
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
	)
}

func (ctrl *TripController) computeTrip() *service.ComputeTripService {
	return service.NewComputeTripService(
		mapsservice.NewDirectionService(ctrl.goong, ctrl.previewLegs()),
		repository.NewTravelRepository(ctrl.db),
		repository.NewDestinationRepository(ctrl.db),
		repository.NewLocationRepository(ctrl.db),
		nil,
	)
}

// computeStoredTrip routes the saved graph. Legs go through Redis and Postgres,
// unlike the preview helper, which keeps a 15 minute Redis entry and nothing else.
// readStoredTrip loads travels already written for the saved graph. It does not route.
func (ctrl *TripController) readStoredTrip() *service.ComputeTripService {
	return service.NewComputeTripService(
		nil,
		repository.NewTravelRepository(ctrl.db),
		nil,
		nil,
		repository.NewTripBranchRepository(ctrl.db),
	)
}

func (ctrl *TripController) computeStoredTrip() *service.ComputeTripService {
	return service.NewComputeTripService(
		mapsservice.NewDirectionService(ctrl.goong, ctrl.durableLegs()),
		repository.NewTravelRepository(ctrl.db),
		nil,
		nil,
		repository.NewTripBranchRepository(ctrl.db),
	)
}

func (ctrl *TripController) durableLegs() mapsrepo.LegStore {
	var memory mapsrepo.LegStore
	if ctrl.redisClient != nil {
		memory = mapsrepo.NewLocationLegMemoryStore(ctrl.redisClient)
	}
	return mapsrepo.NewCachedLegStore(memory, mapsrepo.NewLegRepository(ctrl.db))
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
		errors.Is(err, repository.ErrUserNotFound),
		errors.Is(err, repository.ErrInviteNotFound),
		errors.Is(err, repository.ErrInvitationNotFound),
		errors.Is(err, repository.ErrJoinRequestNotFound),
		errors.Is(err, repository.ErrDestinationNotFound),
		errors.Is(err, repository.ErrLocationNotFound),
		errors.Is(err, repository.ErrBranchNotFound),
		errors.Is(err, repository.ErrBranchStopNotFound):
		jsonError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrAlreadyTripMember),
		errors.Is(err, repository.ErrAlreadyInvited),
		errors.Is(err, repository.ErrJoinRequestPending),
		errors.Is(err, repository.ErrInvitationNotPending),
		errors.Is(err, repository.ErrJoinRequestNotPending),
		errors.Is(err, repository.ErrTripMemberLimit),
		errors.Is(err, repository.ErrNoSuccessorToTransfer),
		errors.Is(err, repository.ErrDestinationNotEditing),
		errors.Is(err, repository.ErrDestinationInUse):
		jsonError(c, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrUserNotTripMember),
		errors.Is(err, repository.ErrCannotKickLeader),
		errors.Is(err, repository.ErrCannotKickSelf),
		errors.Is(err, repository.ErrInsufficientKickRole),
		errors.Is(err, repository.ErrCannotUpdateOtherNickname),
		errors.Is(err, NoPermissionAssignBranch),
		errors.Is(err, repository.ErrCannotChangeLeaderRole):
		jsonError(c, http.StatusForbidden, err.Error())
	case errors.Is(err, repository.ErrNoTripUpdate),
		errors.Is(err, repository.ErrNoTripMemberUpdate),
		errors.Is(err, repository.ErrNoBranchStopUpdate),
		errors.Is(err, repository.ErrTripMembersRequired),
		errors.Is(err, repository.ErrTripTypePolicyUnset),
		errors.Is(err, repository.ErrCannotInviteSelf):
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

func parseAcceptOrReject(c *gin.Context) (string, bool) {
	action := strings.ToLower(strings.TrimSpace(c.Query("action")))
	switch action {
	case membershipActionAccept, membershipActionReject:
		return action, true
	default:
		jsonError(c, http.StatusBadRequest, "action must be accept or reject")
		return "", false
	}
}

func parseTripID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "tripId")
}

func parseUserID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "userId")
}

func parseLocationID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "locationId")
}

func parseDestinationID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "destinationId")
}

func parseBranchID(c *gin.Context) (uuid.UUID, bool) {
	return parsePathID(c, "branchId")
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
