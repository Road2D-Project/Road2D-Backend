package controller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/chat/model/response"
	"Road-To-Destination-BE/module/chat/repository"
	"Road-To-Destination-BE/module/chat/service"
	"Road-To-Destination-BE/module/share"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func currentActor(c *gin.Context) (uuid.UUID, bool) {
	c.Header("Cache-Control", "private, no-store")
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Unauthorized"))
		return uuid.Nil, false
	}
	return user.ID, true
}

func conversationID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("conversationId"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "invalid conversationId"))
		return uuid.Nil, false
	}
	return id, true
}

func mapChatError(c *gin.Context, err error) {
	var expired *repository.ReplayExpiredError
	if errors.As(err, &expired) {
		c.JSON(http.StatusGone, response.ReplayRequiredResponse{ErrorResponse: share.NewError(http.StatusGone, expired.Error()),
			SyncRequired: true, MinAfterEventSeq: expired.MinAfterEventSeq, LatestEventSeq: expired.LatestEventSeq})
		return
	}
	status, message := http.StatusServiceUnavailable, "Chat temporarily unavailable"
	if errors.Is(err, repository.ErrConversationNotFound) || errors.Is(err, repository.ErrMessageNotFound) {
		status, message = http.StatusNotFound, err.Error()
	} else if errors.Is(err, service.ErrInvalidQuery) || errors.Is(err, service.ErrInvalidInput) || errors.Is(err, repository.ErrInvalidCursor) {
		status, message = http.StatusBadRequest, err.Error()
	} else if errors.Is(err, service.ErrForbidden) {
		status, message = http.StatusForbidden, err.Error()
	} else if errors.Is(err, service.ErrConflict) || errors.Is(err, service.ErrIdempotencyConflict) {
		status, message = http.StatusConflict, err.Error()
	}
	c.JSON(status, share.NewError(status, message))
}

func messageID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("messageId"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "invalid messageId"))
		return uuid.Nil, false
	}
	return id, true
}

func bindChatJSON(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err == nil {
		var extra any
		if next := decoder.Decode(&extra); next != io.EOF {
			err = next
			if err == nil {
				err = service.ErrInvalidInput
			}
		}
	}
	if err == nil {
		return true
	}
	var oversized *http.MaxBytesError
	if errors.As(err, &oversized) {
		c.JSON(http.StatusRequestEntityTooLarge, share.NewError(http.StatusRequestEntityTooLarge, "chat request too large"))
	} else {
		mapChatError(c, service.ErrInvalidInput)
	}
	return false
}
