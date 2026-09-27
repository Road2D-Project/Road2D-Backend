package response

import (
	"time"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// TravelResponse is one stored hop. Associations on the row are left off.
type TravelResponse struct {
	ID                uuid.UUID    `json:"id" swaggertype:"string" format:"uuid"`
	FromDestinationID uuid.UUID    `json:"fromDestinationId" swaggertype:"string" format:"uuid"`
	ToDestinationID   uuid.UUID    `json:"toDestinationId" swaggertype:"string" format:"uuid"`
	LegID             *uuid.UUID   `json:"legId,omitempty" swaggertype:"string" format:"uuid"`
	Vehicle           enum.Vehicle `json:"vehicle" swaggertype:"string" example:"bike"`
	Polyline          string       `json:"polyline"`
	DistanceM         float64      `json:"distanceM"`
	DurationS         float64      `json:"durationS"`
	IsFrozen          bool         `json:"isFrozen"`
	LastComputedAt    time.Time    `json:"lastComputedAt"`
}

// ComputeTripResponse is the saved graph after routing, one slice per branch.
type ComputeTripResponse struct {
	Branches [][]TravelResponse `json:"branches"`
}

func FromTravelGraph(graph *model.TravelGraph) ComputeTripResponse {
	if graph == nil {
		return ComputeTripResponse{Branches: [][]TravelResponse{}}
	}
	branches := make([][]TravelResponse, len(*graph))
	for i, row := range *graph {
		branches[i] = make([]TravelResponse, len(row))
		for j, travel := range row {
			branches[i][j] = TravelResponse{
				ID:                travel.ID,
				FromDestinationID: travel.FromDestinationID,
				ToDestinationID:   travel.ToDestinationID,
				LegID:             travel.LegID,
				Vehicle:           travel.Vehicle,
				Polyline:          travel.Polyline,
				DistanceM:         travel.DistanceM,
				DurationS:         travel.DurationS,
				IsFrozen:          travel.IsFrozen,
				LastComputedAt:    travel.LastComputedAt,
			}
		}
	}
	return ComputeTripResponse{Branches: branches}
}
