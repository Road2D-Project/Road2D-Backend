package response

import "github.com/google/uuid"

// PreviewLocation is one catalog place on a reviewed main branch.
type PreviewLocation struct {
	LocationID uuid.UUID `json:"locationId" swaggertype:"string" format:"uuid"`
	Name       string    `json:"name"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
}

// PreviewLocationsResponse is the reviewed route of ordered locations.
// The same object is the optional mainBranch on create: that branch is the
// trip's initial main branch, forked into destinations with these hops stored.
type PreviewLocationsResponse struct {
	Locations []PreviewLocation  `json:"locations"`
	Legs      []ComputeBranchLeg `json:"legs"`
}
