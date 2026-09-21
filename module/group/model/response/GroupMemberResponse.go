package response

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

type GroupMemberResponse struct {
	UserId      uuid.UUID             `json:"userId" swaggertype:"string" format:"uuid"`
	UserName    string                `json:"userName"`
	Nickname    string                `json:"nickname"`
	Role        enum.GroupRole        `json:"role" swaggertype:"string" example:"member"`
	Status      enum.MembershipStatus `json:"status" swaggertype:"string" example:"active"`
	InvitorName string                `json:"invitorName,omitempty"`
	JoinedAt    *time.Time            `json:"joinedAt,omitempty"`
}

type GroupMemberListResponse struct {
	Members []GroupMemberResponse `json:"members"`
}

func MemberFromGroupMember(member *model.GroupMember) GroupMemberResponse {
	if member == nil {
		return GroupMemberResponse{}
	}
	out := GroupMemberResponse{
		UserId:   member.UserID,
		Nickname: member.Nickname,
		Role:     member.Role,
		Status:   member.Status,
		JoinedAt: member.JoinedAt,
	}
	if member.User != nil {
		out.UserName = member.User.Username
	}
	if member.InvitorName != nil {
		out.InvitorName = *member.InvitorName
	}
	return out
}
