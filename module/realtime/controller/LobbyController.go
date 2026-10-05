package controller

import (
	"context"
	"net/http"

	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var _ share.RouterRegistrar = (*LobbyController)(nil)

// RoomAuthorizer decides whether an authenticated user may enter a room.
// The realtime package does not know about trips. The process wires the check
// (active trip membership today). Return realtime.ErrNotMember for a denial.
type RoomAuthorizer interface {
	AuthorizeRoom(ctx context.Context, userID uuid.UUID, roomID string) error
}

// LobbyController upgrades an authenticated request into a room connection.
type LobbyController struct {
	auth       *middleware.AuthenticationMiddleware
	hub        *realtime.Hub
	authorizer RoomAuthorizer
}

func NewLobbyController(auth *middleware.AuthenticationMiddleware, hub *realtime.Hub, authorizer RoomAuthorizer) *LobbyController {
	return &LobbyController{auth: auth, hub: hub, authorizer: authorizer}
}

func (ctrl *LobbyController) RegisterRoutes(router *gin.RouterGroup) {
	rooms := router.Group("/realtime/rooms", liftAccessToken(), ctrl.auth.RequireAuth())
	rooms.GET("/:roomId", ctrl.HandleJoinRoom())
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Browsers send Origin on the upgrade. Restrict this to the app origin
	// before the lobby is exposed beyond local development.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// liftAccessToken copies ?access_token= into Authorization when the header is
// absent. The browser WebSocket API cannot set that header. Header auth still wins.
func liftAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			if token := c.Query("access_token"); token != "" {
				c.Request.Header.Set("Authorization", "Bearer "+token)
			}
		}
		c.Next()
	}
}
