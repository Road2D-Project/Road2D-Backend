package controller

import "github.com/gin-gonic/gin"

// HandleJoinPersonal upgrades the authenticated user's own event channel.
// @Summary Open the personal events WebSocket
// @Description Shared channel for conversation badges and future notifications. The server derives its room from the authenticated user.
// @Tags Chatting
// @Security BearerAuth
// @Success 101 "WebSocket upgraded"
// @Failure 401,403,503 {object} share.ErrorResponse
// @Router /realtime/me [get]
func (ctrl *ChattingController) HandleJoinPersonal() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, options, ok := ctrl.connectionIdentity(c)
		if !ok {
			return
		}
		ctrl.upgrade(c, ctrl.rooms.Personal, actor.String(), actor, options)
	}
}
