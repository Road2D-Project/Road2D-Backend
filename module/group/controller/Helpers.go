package controller

import (
	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	authRepo "Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/module/group/service"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/utils/customValidator"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (ctrl *GroupController) groupService() *service.GroupService {
	return service.NewGroupService(
		repository.NewGroupRepository(ctrl.db),
		authRepo.NewUserRepository(ctrl.db),
		repository.NewGroupMemberRepository(ctrl.db),
		repository.NewGroupMemberStore(ctrl.redisClient),
	)
}

func (ctrl *GroupController) inviteNewMemberService() *service.InviteNewMemberService {
	return service.NewInviteNewMemberService(repository.NewGroupMemberRepository(ctrl.db))
}

func (ctrl *GroupController) invitationRespondingService() *service.InvitationRespondingService {
	return service.NewInvitationRespondingService(
		repository.NewGroupMemberRepository(ctrl.db),
		repository.NewGroupMemberStore(ctrl.redisClient),
	)
}

func mapGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrGroupNotFound),
		errors.Is(err, repository.ErrUserNotFound),
		errors.Is(err, repository.ErrInvitationNotFound):
		jsonError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrGroupHasTrips),
		errors.Is(err, repository.ErrAlreadyGroupMember),
		errors.Is(err, repository.ErrAlreadyInvited),
		errors.Is(err, repository.ErrJoinRequestPending),
		errors.Is(err, repository.ErrInvitationNotPending):
		jsonError(c, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrUserNotGroupMember):
		jsonError(c, http.StatusForbidden, err.Error())
	case errors.Is(err, repository.ErrAdminUserNotFound),
		errors.Is(err, repository.ErrNoGroupUpdate),
		errors.Is(err, repository.ErrCannotInviteSelf):
		jsonError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrInternalServerError):
		jsonError(c, http.StatusInternalServerError, err.Error())
	default:
		jsonError(c, http.StatusBadRequest, err.Error())
	}
}

func parseGroupID(c *gin.Context) (uuid.UUID, bool) {
	id, err := parsePathUUID(c, "groupId", "invalid groupId")
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func parsePathUUID(c *gin.Context, param, invalidMessage string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		jsonError(c, http.StatusBadRequest, invalidMessage)
		return uuid.Nil, err
	}
	return id, nil
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
