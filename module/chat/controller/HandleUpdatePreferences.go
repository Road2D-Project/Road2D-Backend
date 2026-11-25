package controller

import (
	"Road-To-Destination-BE/module/chat/model/request"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleUpdatePreferences sets or clears the caller's mute deadline, not read state.
// @Summary Set conversation mute preference
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation UUID"
// @Param body body request.UpdatePreferencesRequest true "Future mutedUntil timestamp or null"
// @Success 200 {object} response.ConversationResponse
// @Failure 400,401,404,413,503 {object} share.ErrorResponse
// @Router /conversations/{conversationId}/preferences [patch]
func (ctrl *ChatController) HandleUpdatePreferences() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := currentActor(c)
		if !ok {
			return
		}
		id, ok := conversationID(c)
		if !ok {
			return
		}
		var req request.UpdatePreferencesRequest
		if !bindChatJSON(c, &req) {
			return
		}
		out, err := ctrl.state.UpdatePreferences(c.Request.Context(), actor, id, req)
		if err != nil {
			mapChatError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
