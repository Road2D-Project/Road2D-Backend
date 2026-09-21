package response

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

type GroupResponse struct {
	ID          uuid.UUID       `json:"id" swaggertype:"string" format:"uuid"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Policy      string          `json:"policy"`
	OwnerID     uuid.UUID       `json:"ownerId" swaggertype:"string" format:"uuid"`
	MyRole      *enum.GroupRole `json:"myRole,omitempty" swaggertype:"string" example:"owner"`
	CreatedTime time.Time       `json:"createdTime"`
	UpdatedTime time.Time       `json:"updatedTime"`
}

type GroupListResponse struct {
	Groups []GroupResponse `json:"groups"`
}

func FromGroup(group *model.Group, role *enum.GroupRole) GroupResponse {
	return GroupResponse{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		Policy:      group.Policy,
		OwnerID:     group.OwnerID,
		MyRole:      role,
		CreatedTime: group.CreatedAt,
		UpdatedTime: group.UpdatedAt,
	}
}

func FromGroupPtr(group *model.Group, role *enum.GroupRole) *GroupResponse {
	out := FromGroup(group, role)
	return &out
}

func RolePtr(role enum.GroupRole) *enum.GroupRole {
	copied := role
	return &copied
}
