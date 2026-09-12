package model

import (
	"Road-To-Destination-BE/utils"

	"github.com/google/uuid"
)

// Compound is the VN administrative breakdown persisted on Location.
type Compound struct {
	Commune  string `json:"commune,omitempty"`
	District string `json:"district,omitempty"`
	Province string `json:"province,omitempty"`
}

// Location is a verified (or resolvable) place. lat/lng are required.
// place_id / address / compound / PlaceType are optional.
type Location struct {
	utils.Base
	Lat              float64    `json:"lat" gorm:"column:lat;not null"`
	Lng              float64    `json:"lng" gorm:"column:lng;not null"`
	Address          *string    `json:"address,omitempty" gorm:"column:address;type:text"`
	FormattedAddress *string    `json:"formattedAddress,omitempty" gorm:"column:formatted_address;type:text"`
	PlaceID          *string    `json:"placeId,omitempty" gorm:"column:place_id;type:varchar(255);index"`
	Compound         *Compound  `json:"compound,omitempty" gorm:"column:compound;type:jsonb;serializer:json"`
	IsVerified       bool       `json:"isVerified" gorm:"column:is_verified;not null;default:false"`
	PlaceTypeID      *uuid.UUID `json:"placeTypeId,omitempty" gorm:"type:uuid;index"`
	PlaceType        *PlaceType `json:"placeType,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func (Location) TableName() string {
	return "locations"
}
