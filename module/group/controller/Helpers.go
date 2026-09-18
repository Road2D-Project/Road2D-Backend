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

func mapGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrGroupNotFound):
		jsonError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrGroupHasTrips):
		jsonError(c, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrAdminUserNotFound), errors.Is(err, repository.ErrNoGroupUpdate):
		jsonError(c, http.StatusBadRequest, err.Error())
	default:
		jsonError(c, http.StatusBadRequest, err.Error())
	}
}

func parseGroupID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("groupId"))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid groupId")
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
