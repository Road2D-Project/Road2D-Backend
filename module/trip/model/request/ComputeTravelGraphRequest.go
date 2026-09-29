package request

import "github.com/google/uuid"

// ComputeTravelGraphRequest routes a graph of destination ids and stores the travels.
// Each inner slice is one branch in stop order. The service loads those destinations
// and creates a hop between each consecutive pair. Stored branch rows are not the source.
type ComputeTravelGraphRequest struct {
	Branches [][]uuid.UUID `json:"branches" binding:"required,min=1,dive,min=2"`
}
