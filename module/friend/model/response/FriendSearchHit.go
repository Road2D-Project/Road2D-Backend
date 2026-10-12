package response

import "github.com/google/uuid"

// FriendRelation is how the caller is tied to a searched user.
// none covers strangers and a previously rejected request.
type FriendRelation string

const (
	RelationNone     FriendRelation = "none"
	RelationOutgoing FriendRelation = "outgoing"
	RelationIncoming FriendRelation = "incoming"
	RelationFriends  FriendRelation = "friends"
)

// FriendSearchHit is a user the caller may befriend.
// The phone number used to find them is not returned.
type FriendSearchHit struct {
	UserID   uuid.UUID      `json:"userId" swaggertype:"string" format:"uuid"`
	Username string         `json:"username"`
	Relation FriendRelation `json:"relation" example:"none"`
}
