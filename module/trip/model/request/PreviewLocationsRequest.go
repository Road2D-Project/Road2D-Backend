package request

import "github.com/google/uuid"

// PreviewLocationsRequest asks for the bike route along ordered catalog places.
// There is no trip yet, so every stop is a location id.
type PreviewLocationsRequest struct {
	LocationIDs []uuid.UUID `json:"locationIds" binding:"required,min=2,dive,required" swaggertype:"array,string"`
}
