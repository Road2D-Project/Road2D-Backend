package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleGetConversation hides nonexistent and inaccessible conversations alike.
// @Summary Get conversation summary
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Success 200 {object} response.ConversationResponse
// @Failure 400,401,404,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId} [get]
func (ctrl *ChatController) HandleGetConversation() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		id, ok := conversationID(c)
		if !ok {
			return
		}
		out, err := ctrl.reads.GetConversation(c.Request.Context(), actor, id)
		if err != nil {
			mapChatError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
