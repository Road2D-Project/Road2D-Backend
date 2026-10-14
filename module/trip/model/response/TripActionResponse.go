package response

// TripActionResponse acknowledges a leader status command.
type TripActionResponse struct {
	Message string `json:"message" example:"trip started"`
}
