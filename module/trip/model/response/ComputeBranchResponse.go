package response

import (
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// ComputeBranchStop is one end of a preview leg. Only one id is set.
type ComputeBranchStop struct {
	Name          string     `json:"name"`
	DestinationID *uuid.UUID `json:"destinationId,omitempty" swaggertype:"string" format:"uuid"`
	LocationID    *uuid.UUID `json:"locationId,omitempty" swaggertype:"string" format:"uuid"`
}

// ComputeBranchLeg is one computed hop. It is not a stored Travel.
type ComputeBranchLeg struct {
	From      ComputeBranchStop `json:"from"`
	To        ComputeBranchStop `json:"to"`
	Vehicle   enum.Vehicle      `json:"vehicle" swaggertype:"string" example:"bike"`
	Polyline  string            `json:"polyline"`
	DistanceM float64           `json:"distanceM"`
	DurationS float64           `json:"durationS"`
}

// ComputeBranchResponse is the ordered hops of one preview branch.
type ComputeBranchResponse struct {
	Legs []ComputeBranchLeg `json:"legs"`
}
