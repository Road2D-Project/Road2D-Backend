package model

import (
	"time"

	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// GroupMember is a user's membership in a standing group (not a trip crew).
type GroupMember struct {
	utils.Base
	GroupID     uuid.UUID             `json:"groupId" gorm:"type:uuid;uniqueIndex:idx_group_user;index;not null" swaggertype:"string" format:"uuid"`
	Group       *Group                `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
	UserID      uuid.UUID             `json:"userId" gorm:"type:uuid;uniqueIndex:idx_group_user;index;not null" swaggertype:"string" format:"uuid"`
	User        *authModel.User       `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
	Role        enum.GroupRole        `json:"role" gorm:"column:role;type:varchar(16);not null;default:member" swaggertype:"string" example:"member"`
	Status      enum.MembershipStatus `json:"status" gorm:"column:status;type:varchar(16);not null;index;default:pending" swaggertype:"string" example:"active"`
	Nickname    string                `json:"nickname" gorm:"column:nickname;type:varchar(64);not null"`
	InvitorName *string               `json:"invitorName" gorm:"column:invitor_name;type:varchar(64)"`
	JoinedAt    *time.Time            `json:"joinedAt,omitempty" gorm:"column:joined_at"`
}

func (GroupMember) TableName() string {
	return "group_members"
}

func (m GroupMember) IsActiveStaff() bool {
	return m.Status.IsActive() && m.Role.CanManageMembers()
}
