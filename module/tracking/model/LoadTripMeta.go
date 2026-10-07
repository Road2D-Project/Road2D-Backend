package model

import (
	"errors"

	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrTripNotFound = errors.New("trip not found")

type tripRow struct {
	ID     uuid.UUID
	Name   string
	Status enum.TripStatus
}

func LoadTripMeta(db *gorm.DB, out *TripInformation) error {
	var rows []tripRow
	err := db.Raw(`
		SELECT id, name, status
		FROM trips
		WHERE id = ?
	`, out.TripId).Scan(&rows).Error
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrTripNotFound
	}
	out.TripName = rows[0].Name
	out.Status = rows[0].Status
	return nil
}
