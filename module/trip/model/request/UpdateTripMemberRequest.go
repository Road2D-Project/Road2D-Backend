package request

import "Road-To-Destination-BE/utils/enum"

// UpdateTripMemberRequest changes the caller's nickname, or — for the leader
// only — another member's role to admin or member. Leadership is not assigned
// here; it moves when the leader leaves.
type UpdateTripMemberRequest struct {
	Nickname *string        `json:"nickname" binding:"omitempty,min=1,max=64"`
	Role     *enum.TripRole `json:"role" binding:"omitempty" swaggertype:"string" example:"admin"`
}
