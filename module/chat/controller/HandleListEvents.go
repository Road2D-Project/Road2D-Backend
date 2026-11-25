package controller

import (
	"Road-To-Destination-BE/module/chat/model/request"
	"Road-To-Destination-BE/module/chat/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleListEvents replays retained public/own events, never internal cleanup jobs.
// @Summary Replay conversation events
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Param afterEventSeq query int false "Exclusive event cursor, default zero"
// @Param limit query int false "Page size, default 100, maximum 200"
// @Success 200 {object} response.EventListResponse
// @Failure 410 {object} response.ReplayRequiredResponse
// @Failure 400,401,404,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId}/events [get]
func (ctrl *ChatController) HandleListEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		id, ok := conversationID(c)
		if !ok {
			return
		}
		var query request.ListEventsRequest
		if err := c.ShouldBindQuery(&query); err != nil {
			mapChatError(c, service.ErrInvalidQuery)
			return
		}
		out, err := ctrl.events.ListEvents(c.Request.Context(), actor, id, query)
		if err != nil {
			mapChatError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
