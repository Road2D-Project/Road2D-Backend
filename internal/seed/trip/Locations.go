package trip

import (
	"fmt"

	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DestinationLimit is how many catalog places become pins on the seeded route.
const DestinationLimit = 10

type seedLocation struct {
	ID   uuid.UUID
	Name string
}

// listSeedLocations returns the earliest locations, capped so the route stays a short main branch.
func listSeedLocations(db *gorm.DB, limit int) ([]seedLocation, error) {
	var rows []seedLocation
	err := db.Table(model.Location{}.TableName()).
		Select("id", "name").
		Order("created_at ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// seedLocations keeps the caller's order. Missing ids are an error.
// An empty list falls back to the earliest catalog rows.
func seedLocations(db *gorm.DB, ids []uuid.UUID) ([]seedLocation, error) {
	if len(ids) == 0 {
		return listSeedLocations(db, DestinationLimit)
	}
	rows := make([]seedLocation, 0, len(ids))
	for idx, id := range ids {
		var row seedLocation
		err := db.Table(model.Location{}.TableName()).
			Select("id", "name").
			Where("id = ?", id).
			Take(&row).Error
		if err != nil {
			return nil, fmt.Errorf("location %d %s: %w", idx, id, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}
