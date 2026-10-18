package response

import (
	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
)

// DraftDestinationsResponse is the hidden inbox for one trip.
// These pins are not part of the route graph and have no travels.
// Id, description, and color belong to that inbox branch. The graph response
// leaves the draft out, so this is where a client reads them.
type DraftDestinationsResponse struct {
	ID           uuid.UUID      `json:"id,omitempty" swaggertype:"string" format:"uuid"`
	Description  string         `json:"description"`
	Color        string         `json:"color"`
	Destinations []StopResponse `json:"destinations"`
}

// DraftDestinations maps the draft branch's stops in travel order.
func DraftDestinations(branch *model.TripBranch) DraftDestinationsResponse {
	if branch == nil {
		return DraftDestinationsResponse{Destinations: []StopResponse{}}
	}
	stops := make([]StopResponse, 0, len(branch.Stops))
	for i := range branch.Stops {
		stops = append(stops, stopResponse(&branch.Stops[i]))
	}
	return DraftDestinationsResponse{
		ID:           branch.ID,
		Description:  branch.Description,
		Color:        branch.Color,
		Destinations: stops,
	}
}
