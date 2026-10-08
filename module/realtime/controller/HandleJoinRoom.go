package controller

import (
	"errors"
	"log"
	"net/http"
	"time"

	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func (ctrl *LobbyController) HandleJoinRoom() gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomId")
		if roomID == "" {
			c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "room id is required"))
			return
		}
		user := middleware.GetCurrentUser(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Unauthorized"))
			return
		}
		if ctrl.authorizer == nil || ctrl.hub == nil {
			c.JSON(http.StatusInternalServerError, share.NewError(http.StatusInternalServerError, "realtime lobby is not configured"))
			return
		}
		// Membership is checked before Upgrade so a denial is still a normal HTTP status.
		if err := ctrl.authorizer.AuthorizeRoom(c.Request.Context(), user.ID, roomID); err != nil {
			mapLobbyError(c, err)
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			// The handshake failed before a socket existed. The upgrader already
			// wrote the HTTP error, so there is no body left to send.
			log.Printf("realtime: upgrade room %s: %v", roomID, err)
			return
		}
		if err := ctrl.hub.Join(roomID, user.ID, conn); err != nil {
			log.Printf("realtime: join room %s: %v", roomID, err)
			closeUpgraded(conn, err)
		}
	}
}

// closeUpgraded tells the client why the socket is closing. HTTP status is no
// longer available once Upgrade has succeeded.
func closeUpgraded(conn *websocket.Conn, err error) {
	code := websocket.CloseInternalServerErr
	reason := "room unavailable"
	if errors.Is(err, realtime.ErrRoomClosed) {
		code = websocket.CloseTryAgainLater
		reason = "room closed"
	}
	_ = conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Now().Add(time.Second),
	)
	conn.Close()
}

func mapLobbyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, realtime.ErrNotMember):
		c.JSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "not a member of this room"))
	case errors.Is(err, realtime.ErrInvalidRoom):
		c.JSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "invalid room id"))
	default:
		c.JSON(http.StatusInternalServerError, share.NewError(http.StatusInternalServerError, "failed to authorize room"))
	}
}
