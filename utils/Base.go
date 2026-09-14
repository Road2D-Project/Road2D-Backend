package utils

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Base is the UUID primary key + timestamps used by trip tables.
type Base struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey" swaggertype:"string" format:"uuid"`
	CreatedAt time.Time `json:"createdTime" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedTime" gorm:"autoUpdateTime"`
}

func (base *Base) BeforeCreate(_ *gorm.DB) error {
	if base.ID == uuid.Nil {
		base.ID = uuid.New()
	}
	return nil
}
