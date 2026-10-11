package response

import "github.com/google/uuid"

// EnsureLocationResponse is one catalog place resolved from a Goong place_id.
// locationId is the id POST /planning/preview accepts.
type EnsureLocationResponse struct {
	LocationID uuid.UUID `json:"locationId" swaggertype:"string" format:"uuid"`
	PlaceID    string    `json:"placeId"`
	Name       string    `json:"name"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
}
