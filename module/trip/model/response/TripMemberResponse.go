package response

import (
	"time"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type TripMemberResponse struct {
	TripID           uuid.UUID             `json:"tripId" swaggertype:"string" format:"uuid"`
	TripName         string                `json:"tripName,omitempty"`
	UserID           uuid.UUID             `json:"userId" swaggertype:"string" format:"uuid"`
	Username         string                `json:"username,omitempty"`
	AssignedBranchId uuid.UUID             `json:"assignedBranchId,omitempty"`
	Nickname         string                `json:"nickname"`
	Role             enum.TripRole         `json:"role" swaggertype:"string" example:"member"`
	Status           enum.MembershipStatus `json:"status" swaggertype:"string" example:"active"`
	InvitorName      string                `json:"invitorName,omitempty"`
	JoinedAt         *time.Time            `json:"joinedAt,omitempty"`
}

type TripMemberListResponse struct {
	Members []TripMemberResponse `json:"members"`
}

type TripInvitationListResponse struct {
	Invitations []TripMemberResponse `json:"invitations"`
}

type TripJoinRequestListResponse struct {
	JoinRequests []TripMemberResponse `json:"joinRequests"`
}

func FromTripMember(member *model.TripMember) TripMemberResponse {
	if member == nil {
		return TripMemberResponse{}
	}
	out := TripMemberResponse{
		TripID:   member.TripID,
		UserID:   member.UserID,
		Nickname: member.Nickname,
		Role:     member.Role,
		Status:   member.Status,
		JoinedAt: member.JoinedAt,
	}
	if member.AssignedBranchID != nil {
		out.AssignedBranchId = *member.AssignedBranchID
	}
	if member.Trip != nil {
		out.TripName = member.Trip.Name
	}
	if member.User != nil {
		out.Username = member.User.Username
	}
	if member.InvitorName != nil {
		out.InvitorName = *member.InvitorName
	}
	return out
}

func FromTripMemberPtr(member *model.TripMember) *TripMemberResponse {
	out := FromTripMember(member)
	return &out
}
