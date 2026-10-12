package response

// FriendRequestListResponse is incoming or outgoing pending requests.
type FriendRequestListResponse struct {
	Requests []FriendRequestResponse `json:"requests"`
}
