package response

type TripInviteLinkResponse struct {
	Token    string `json:"token"`
	JoinPath string `json:"joinPath" example:"/v1/trips/join/550e8400-e29b-41d4-a716-446655440000"`
}
