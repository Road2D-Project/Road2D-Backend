package controller

import (
	"Road-To-Destination-BE/module/conversation/model/request"
	"Road-To-Destination-BE/module/conversation/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleListMessages returns messages ascending by sequence, including tombstones.
// @Summary Read conversation history
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Param beforeSeq query int false "Older messages, exclusive; cannot combine with afterSeq"
// @Param afterSeq query int false "Catch up after sequence, exclusive; zero starts at the beginning"
// @Param limit query int false "Page size, default 30, maximum 100"
// @Success 200 {object} response.MessageListResponse
// @Failure 400,401,404,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId}/messages [get]
func (ctrl *ChatController) HandleListMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		id, ok := conversationID(c)
		if !ok {
			return
		}
		var query request.ListMessagesRequest
		if err := c.ShouldBindQuery(&query); err != nil {
			mapChatError(c, service.ErrInvalidQuery)
			return
		}
		out, err := ctrl.reads.ListMessages(c.Request.Context(), actor, id, query)
		if err != nil {
			mapChatError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
