package trip

import (
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
