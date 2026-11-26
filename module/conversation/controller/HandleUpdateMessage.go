package controller

import (
	"Road-To-Destination-BE/module/conversation/model/request"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleUpdateMessage edits only the sender's ordinary text at an expected revision.
// @Summary Edit text message
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Param messageId path string true "Message UUID"
// @Param body body request.UpdateMessageRequest true "New text and expectedRevision"
// @Success 200 {object} response.MessageResponse
// @Failure 400,401,403,404,409,413,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId}/messages/{messageId} [patch]
func (ctrl *ChatController) HandleUpdateMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		id, ok := conversationID(c)
		if !ok {
			return
		}
		message, ok := messageID(c)
		if !ok {
			return
		}
		var req request.UpdateMessageRequest
		if !bindChatJSON(c, &req) {
			return
		}
		out, err := ctrl.messages.UpdateMessage(c.Request.Context(), actor, id, message, req)
		if err != nil {
			mapChatError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
