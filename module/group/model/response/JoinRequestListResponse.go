package response

type JoinRequestListResponse struct {
	JoinRequests []InvitationResponse `json:"joinRequests"`
}
