package model

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/utils"
	"time"

	"github.com/google/uuid"
)

// ConversationUserState is a read/preference record, not proof of membership.
type ConversationUserState struct {
	utils.Base
	ConversationID uuid.UUID       `json:"conversationId" gorm:"type:uuid;not null;uniqueIndex:idx_conversation_user"`
	Conversation   *Conversation   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	UserID         uuid.UUID       `json:"userId" gorm:"type:uuid;not null;uniqueIndex:idx_conversation_user;index"`
	User           *authModel.User `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	LastReadSeq    int64           `json:"lastReadSeq" gorm:"not null;default:0;check:chk_chat_read_seq,last_read_seq >= 0"`
	MutedUntil     *time.Time      `json:"mutedUntil,omitempty"`
}

func (ConversationUserState) TableName() string { return "conversation_user_states" }
