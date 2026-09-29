package response

import "Road-To-Destination-BE/module/trip/model"

// DraftDestinationsResponse is the hidden inbox for one trip.
// These pins are not part of the route graph and have no travels.
type DraftDestinationsResponse struct {
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
	return DraftDestinationsResponse{Destinations: stops}
}
