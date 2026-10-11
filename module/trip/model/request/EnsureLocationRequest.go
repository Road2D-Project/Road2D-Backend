package request

// EnsureLocationRequest stores a Goong place in the shared catalog.
// placeId comes from autocomplete. The response locationId is what preview accepts.
type EnsureLocationRequest struct {
	PlaceID string `json:"placeId" binding:"required"`
}
