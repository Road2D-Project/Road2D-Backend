package model

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

// Message keeps its position and reply identity when its content is tombstoned.
// The migration enforces replies within the same conversation using a composite FK.
type Message struct {
	utils.Base
	ConversationID   uuid.UUID        `json:"conversationId" gorm:"type:uuid;not null;uniqueIndex:idx_message_seq;uniqueIndex:idx_message_client"`
	Conversation     *Conversation    `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SenderID         *uuid.UUID       `json:"senderId,omitempty" gorm:"type:uuid;uniqueIndex:idx_message_client"`
	Sender           *authModel.User  `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Seq              int64            `json:"seq" gorm:"not null;uniqueIndex:idx_message_seq;check:chk_message_seq,seq > 0"`
	Kind             enum.MessageKind `json:"kind" gorm:"type:varchar(16);not null;check:chk_message_kind,kind IN ('text','media','poll','system')" swaggertype:"string"`
	Text             *string          `json:"text,omitempty" gorm:"type:text"`
	ReplyToMessageID *uuid.UUID       `json:"replyToMessageId,omitempty" gorm:"type:uuid"`
	ClientMessageID  *uuid.UUID       `json:"-" gorm:"type:uuid;uniqueIndex:idx_message_client"`
	RequestHash      string           `json:"-" gorm:"type:varchar(64);not null;default:''"`
	Revision         int64            `json:"revision" gorm:"not null;default:1;check:chk_message_revision,revision > 0"`
	EditedAt         *time.Time       `json:"editedAt,omitempty"`
	DeletedAt        *time.Time       `json:"deletedAt,omitempty"`
}

func (Message) TableName() string { return "messages" }
