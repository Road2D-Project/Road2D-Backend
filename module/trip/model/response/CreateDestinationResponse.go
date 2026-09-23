package response

import "github.com/google/uuid"

type CreateDestinationResponse struct {
	DestinationId uuid.UUID  `json:"destinationId"`
	LocationId    *uuid.UUID `json:"locationId;omitempty"`
	PlaceId       *string    `json:"placeId;omitempty"`
}
