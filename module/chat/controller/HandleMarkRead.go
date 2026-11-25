package controller

import (
	"Road-To-Destination-BE/module/chat/model/request"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleMarkRead advances only the caller's read cursor.
// @Summary Mark displayed messages read
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Param body body request.MarkReadRequest true "Highest displayed sequence"
// @Success 200 {object} response.ConversationResponse
// @Failure 400,401,404,413,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId}/read [put]
func (ctrl *ChatController) HandleMarkRead() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		id, ok := conversationID(c)
		if !ok {
			return
		}
		var req request.MarkReadRequest
		if !bindChatJSON(c, &req) {
			return
		}
		out, err := ctrl.state.MarkRead(c.Request.Context(), actor, id, req)
		if err != nil {
			mapChatError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
