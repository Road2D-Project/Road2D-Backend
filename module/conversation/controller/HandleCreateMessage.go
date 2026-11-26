package controller

import (
	"Road-To-Destination-BE/module/conversation/model/request"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleCreateMessage persists a text message and outbox event atomically.
// @Summary Send text message
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Param body body request.CreateMessageRequest true "Client UUID, text and optional reply"
// @Success 201,200 {object} response.MessageResponse
// @Failure 400,401,404,409,413,429,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId}/messages [post]
func (ctrl *ChatController) HandleCreateMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		id, ok := conversationID(c)
		if !ok {
			return
		}
		var req request.CreateMessageRequest
		if !bindChatJSON(c, &req) {
			return
		}
		out, created, err := ctrl.messages.CreateMessage(c.Request.Context(), actor, id, req)
		if err != nil {
			mapChatError(c, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		c.JSON(status, out)
	}
}
