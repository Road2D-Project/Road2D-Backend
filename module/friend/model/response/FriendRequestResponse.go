package response

import (
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// FriendRequestResponse is one pending, accepted, or rejected friendship
// as seen by the caller. UserID is the other user.
type FriendRequestResponse struct {
	UserID   uuid.UUID         `json:"userId" swaggertype:"string" format:"uuid"`
	Username string            `json:"username"`
	Status   enum.FriendStatus `json:"status" swaggertype:"string" example:"pending"`
}
