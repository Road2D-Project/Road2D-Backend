package request

import (
	"time"

	"github.com/google/uuid"
)

type CreateTripRequest struct {
	GroupID   uuid.UUID  `json:"groupId" binding:"required" swaggertype:"string" format:"uuid"`
	Name      string     `json:"name" binding:"required,min=1,max=255"`
	Note      string     `json:"note" binding:"max=4000"`
	StartTime *time.Time `json:"startTime" binding:"omitempty"`
	EndTime   *time.Time `json:"endTime" binding:"omitempty"`
}
