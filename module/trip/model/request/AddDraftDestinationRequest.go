package request

import "github.com/google/uuid"

// AddDraftDestinationRequest parks one already created pin on the trip's draft inbox.
type AddDraftDestinationRequest struct {
	DestinationID uuid.UUID `json:"destinationId" binding:"required" swaggertype:"string" format:"uuid"`
}
