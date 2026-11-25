package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleDeleteMessage tombstones a text message without changing its sequence.
// @Summary Delete text message
// @Tags chat
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Param messageId path string true "Message UUID"
// @Success 204 "Message tombstoned"
// @Failure 400,401,403,404,409,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId}/messages/{messageId} [delete]
func (ctrl *ChatController) HandleDeleteMessage() gin.HandlerFunc {
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
		if err := ctrl.messages.DeleteMessage(c.Request.Context(), actor, id, message); err != nil {
			mapChatError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
