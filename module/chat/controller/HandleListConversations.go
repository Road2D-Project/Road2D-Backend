package controller

import (
	"Road-To-Destination-BE/module/chat/model/request"
	"Road-To-Destination-BE/module/chat/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleListConversations lists only conversations from the caller's active trips.
// @Summary List my conversations
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param cursor query string false "Opaque activity/ID cursor"
// @Param limit query int false "Page size, default 20, maximum 50"
// @Success 200 {object} response.ConversationListResponse
// @Failure 400,401,503 {object} share.ErrorResponse
// @Router /conversations [get]
func (ctrl *ChatController) HandleListConversations() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		var query request.ListConversationsRequest
		if err := c.ShouldBindQuery(&query); err != nil {
			mapChatError(c, service.ErrInvalidQuery)
			return
		}
		out, err := ctrl.reads.ListConversations(c.Request.Context(), actor, query)
		if err != nil {
			mapChatError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
