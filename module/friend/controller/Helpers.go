package controller

import (
	"errors"
	"net/http"
	"strings"

	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/friend/repository"
	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func mapFriendError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrUserNotFound),
		errors.Is(err, repository.ErrRequestNotFound),
		errors.Is(err, repository.ErrNotFriends):
		jsonError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrAlreadyFriends),
		errors.Is(err, repository.ErrRequestAlreadySent):
		jsonError(c, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrNotRequestAddressee):
		jsonError(c, http.StatusForbidden, err.Error())
	case errors.Is(err, repository.ErrCannotFriendSelf),
		errors.Is(err, repository.ErrEmptySearch),
		errors.Is(err, repository.ErrSearchQueryTooLong):
		jsonError(c, http.StatusBadRequest, err.Error())
	default:
		jsonError(c, http.StatusBadRequest, err.Error())
	}
}

func parseAcceptOrReject(c *gin.Context) (bool, bool) {
	action := strings.ToLower(strings.TrimSpace(c.Query("action")))
	switch action {
	case "accept":
		return true, true
	case "reject":
		return false, true
	default:
		jsonError(c, http.StatusBadRequest, "action must be accept or reject")
		return false, false
	}
}

func parseRequestBox(c *gin.Context) (bool, bool) {
	box := strings.ToLower(strings.TrimSpace(c.Query("box")))
	switch box {
	case "", "in":
		return true, true
	case "out":
		return false, true
	default:
		jsonError(c, http.StatusBadRequest, "box must be in or out")
		return false, false
	}
}

func parseUserID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid userId")
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
