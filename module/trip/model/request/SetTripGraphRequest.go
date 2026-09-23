package request

import "github.com/google/uuid"

// SetTripGraphRequest replaces the whole route graph of a trip.
// Branches[0] is the main branch; every later branch is a sub branch whose
// first id is the split point and whose last id is the merge point.
// OpenTail[i] marks sub branch i as ending without rejoining, so its last id
// does not have to appear in another branch. OpenTail[0] must be false.
type SetTripGraphRequest struct {
	Branches [][]uuid.UUID `json:"branches" binding:"required,min=1,dive,min=1"`
	OpenTail []bool        `json:"openTail" binding:"required"`
}
