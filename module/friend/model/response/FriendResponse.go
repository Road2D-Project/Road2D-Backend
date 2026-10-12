package response

import "github.com/google/uuid"

// FriendResponse is one accepted friend. Phone, email, and password are omitted.
type FriendResponse struct {
	UserID   uuid.UUID `json:"userId" swaggertype:"string" format:"uuid"`
	Username string    `json:"username"`
}
