package controller

import (
	"context"

	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/chatting"
	"Road-To-Destination-BE/module/conversation/model/response"
	"Road-To-Destination-BE/module/share"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ConversationReader reuses the same authoritative active-membership check as
// REST. The summary is discarded at admission; no room callback performs IO.
type ConversationReader interface {
	GetConversation(context.Context, uuid.UUID, uuid.UUID) (*response.ConversationResponse, error)
}

var _ share.RouterRegistrar = (*ChattingController)(nil)

type ChattingController struct {
	auth     *middleware.AuthenticationMiddleware
	reads    ConversationReader
	rooms    *chatting.ChattingHubs
	upgrader websocket.Upgrader
}

func NewChattingController(auth *middleware.AuthenticationMiddleware, reads ConversationReader, rooms *chatting.ChattingHubs, origins []string) (*ChattingController, error) {
	if auth == nil || reads == nil || rooms == nil || rooms.Conversations == nil || rooms.Personal == nil {
		return nil, ErrConfiguration
	}
	upgrader, err := newUpgrader(origins)
	if err != nil {
		return nil, err
	}
	return &ChattingController{auth: auth, reads: reads, rooms: rooms, upgrader: upgrader}, nil
}

func (ctrl *ChattingController) RegisterRoutes(router *gin.RouterGroup) {
	live := router.Group("/realtime", ctrl.guardAdmission(), ctrl.auth.RequireAuth())
	live.GET("/me", ctrl.HandleJoinPersonal())
	live.GET("/conversations/:conversationId", ctrl.HandleJoinConversation())
}

func (ctrl *ChattingController) guardAdmission() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ctrl.rooms.WithAccess(c.Request.Context(), func() error { c.Next(); return nil }); err != nil {
			c.Abort()
			mapChattingError(c, err)
		}
	}
}
