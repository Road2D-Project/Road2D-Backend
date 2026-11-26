package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/share"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HandleJoinConversation upgrades only a current active trip member.
// @Summary Open the active conversation WebSocket
// @Description Pushes sync hints and transient typing. Durable writes use REST. Browser clients may authenticate with access_token as described in docs/chatting.md.
// @Tags Chatting
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Success 101 "WebSocket upgraded"
// @Failure 400,401,403,404,503 {object} share.ErrorResponse
// @Router /realtime/conversations/{conversationId} [get]
func (ctrl *ChattingController) HandleJoinConversation() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, options, ok := ctrl.connectionIdentity(c)
		if !ok {
			return
		}
		id, err := uuid.Parse(c.Param("conversationId"))
		if err != nil || id == uuid.Nil {
			c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "invalid conversationId"))
			return
		}
		conversation, err := ctrl.reads.GetConversation(c.Request.Context(), actor, id)
		if err != nil {
			mapChattingError(c, err)
			return
		}
		if conversation == nil || conversation.ID != id {
			mapChattingError(c, ErrConfiguration)
			return
		}
		ctrl.upgrade(c, ctrl.rooms.Conversations, id.String(), actor, options)
	}
}
