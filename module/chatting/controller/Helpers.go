package controller

import (
	"errors"
	"log"
	"net/http"
	"time"

	"Road-To-Destination-BE/middleware"
	conversationRepository "Road-To-Destination-BE/module/conversation/repository"
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/share"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (ctrl *ChattingController) connectionIdentity(c *gin.Context) (uuid.UUID, realtime.ConnectionOptions, bool) {
	c.Header("Cache-Control", "private, no-store")
	user := middleware.GetCurrentUser(c)
	session, exists := middleware.GetAccessSession(c)
	options := realtime.ConnectionOptions{
		TokenID: session.TokenID, SessionVersion: session.SessionVersion, ExpiresAt: session.ExpiresAt,
	}
	if user == nil || user.ID == uuid.Nil || !exists || options.Validate() != nil {
		c.JSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Invalid access session"))
		return uuid.Nil, options, false
	}
	if !ctrl.upgrader.CheckOrigin(c.Request) {
		c.JSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "WebSocket origin is not allowed"))
		return uuid.Nil, options, false
	}
	return user.ID, options, true
}

func (ctrl *ChattingController) upgrade(c *gin.Context, hub *realtime.Hub, roomID string, actor uuid.UUID, options realtime.ConnectionOptions) {
	conn, err := ctrl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrader already wrote the handshake error.
	}
	if err := hub.JoinWithOptions(roomID, actor, conn, options); err != nil {
		log.Printf("chatting: join room %s: %v", roomID, err)
		code, reason := websocket.CloseInternalServerErr, "room unavailable"
		if errors.Is(err, realtime.ErrConnectionExpired) {
			code, reason = websocket.ClosePolicyViolation, "access token expired"
		} else if errors.Is(err, realtime.ErrRoomClosed) {
			code, reason = websocket.CloseTryAgainLater, "room closed"
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
		_ = conn.Close()
	}
}

func mapChattingError(c *gin.Context, err error) {
	status, message := http.StatusServiceUnavailable, "Chatting temporarily unavailable"
	if errors.Is(err, conversationRepository.ErrConversationNotFound) {
		status, message = http.StatusNotFound, "conversation not found"
	}
	c.JSON(status, share.NewError(status, message))
}
