package response

// FriendListResponse is the caller's accepted friends.
// Count is the length of Friends after an optional username filter.
type FriendListResponse struct {
	Count   int              `json:"count"`
	Friends []FriendResponse `json:"friends"`
}
