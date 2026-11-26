package model

import (
	tripModel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils"

	"github.com/google/uuid"
)

// Conversation derives its roster and permissions from its trip, never a chat roster.
type Conversation struct {
	utils.Base
	TripID         uuid.UUID       `json:"tripId" gorm:"type:uuid;not null;uniqueIndex:idx_conversation_trip"`
	Trip           *tripModel.Trip `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	NextMessageSeq int64           `json:"-" gorm:"not null;default:1;check:chk_conversation_message_seq,next_message_seq > 0"`
	NextEventSeq   int64           `json:"-" gorm:"not null;default:1;check:chk_conversation_event_seq,next_event_seq > 0"`
	ReplayFloor    int64           `json:"-" gorm:"not null;default:0;check:chk_conversation_replay_floor,replay_floor >= 0 AND replay_floor < next_event_seq"`
}

func (Conversation) TableName() string { return "conversations" }

func NewConversation(tripID uuid.UUID) *Conversation {
	return &Conversation{TripID: tripID, NextMessageSeq: 1, NextEventSeq: 1}
}
