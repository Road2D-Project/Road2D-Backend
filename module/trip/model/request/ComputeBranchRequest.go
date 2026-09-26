package request

import "github.com/google/uuid"

// ComputeBranchPoint is one stop on a preview branch.
// Exactly one of the two ids is set: a trip pin or a verified place.
type ComputeBranchPoint struct {
	DestinationID *uuid.UUID `json:"destinationId,omitempty" swaggertype:"string" format:"uuid"`
	LocationID    *uuid.UUID `json:"locationId,omitempty" swaggertype:"string" format:"uuid"`
}

// ComputeBranchRequest asks for the route along one ordered branch.
// The points are not saved and do not have to be the trip's stored graph.
type ComputeBranchRequest struct {
	Points []ComputeBranchPoint `json:"points" binding:"required,min=2,dive"`
}
