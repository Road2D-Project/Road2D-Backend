package response

// FriendSearchResponse is a short ranked list, not the full user table.
type FriendSearchResponse struct {
	Users []FriendSearchHit `json:"users"`
}
