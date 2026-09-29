package request

import "github.com/google/uuid"

// UpdateBranchStopRequest changes one stop already on a branch.
// DestinationID replaces the pin at that stop. OrderInBranch moves it within
// the same branch; zero is the first stop, so the field is a pointer.
type UpdateBranchStopRequest struct {
	DestinationID *uuid.UUID `json:"destinationId,omitempty" swaggertype:"string" format:"uuid"`
	OrderInBranch *int       `json:"orderInBranch,omitempty"`
}
