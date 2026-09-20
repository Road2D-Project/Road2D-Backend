package response

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type InvitationResponse struct {
	GroupId     uuid.UUID             `json:"groupId" swaggertype:"string" format:"uuid"`
	GroupName   string                `json:"groupName"`
	UserId      uuid.UUID             `json:"userId" swaggertype:"string" format:"uuid"`
	UserName    string                `json:"userName"`
	UserRole    *enum.GroupRole       `json:"userRole,omitempty" swaggertype:"string" example:"member"`
	Status      enum.MembershipStatus `json:"status" swaggertype:"string" example:"invited"`
	InvitorName string                `json:"invitorName"`
}

type InvitationListResponse struct {
	Invitations []InvitationResponse `json:"invitations"`
}

func FromGroupMember(member *model.GroupMember) InvitationResponse {
	if member == nil {
		return InvitationResponse{}
	}
	out := InvitationResponse{
		GroupId:  member.GroupID,
		UserId:   member.UserID,
		UserRole: RolePtr(member.Role),
		Status:   member.Status,
	}
	if member.Group != nil {
		out.GroupName = member.Group.Name
	}
	if member.User != nil {
		out.UserName = member.User.Username
	}
	if member.InvitorName != nil {
		out.InvitorName = *member.InvitorName
	}
	return out
}

func FromGroupMemberPtr(member *model.GroupMember) *InvitationResponse {
	out := FromGroupMember(member)
	return &out
}
