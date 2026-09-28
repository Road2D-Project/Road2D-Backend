package response

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// TripDetailResponse is a trip plus its route graph, without the audit
// columns that the persisted branch and stop rows carry.
type TripDetailResponse struct {
	TripResponse
	Branches []BranchResponse `json:"branches"`
}

// BranchResponse is one branch. The split and merge ids are kept apart from
// the stops because an open-ended branch has no merge even though it still
// has a last stop.
type BranchResponse struct {
	ID                     uuid.UUID         `json:"id" swaggertype:"string" format:"uuid"`
	Label                  string            `json:"label"`
	SplitFromDestinationID *uuid.UUID        `json:"splitFromDestinationId,omitempty" swaggertype:"string" format:"uuid"`
	MergeToDestinationID   *uuid.UUID        `json:"mergeToDestinationId,omitempty" swaggertype:"string" format:"uuid"`
	Stops                  []StopResponse    `json:"stops"`
	Travels                []*TravelResponse `json:"travels,omitempty"`
}

// StopResponse is one ordered stop, reduced to what the client renders.
type StopResponse struct {
	DestinationID uuid.UUID `json:"destinationId" swaggertype:"string" format:"uuid"`
	Name          string    `json:"name"`
	Lat           float64   `json:"lat"`
	Lng           float64   `json:"lng"`
	OrderInBranch int       `json:"orderInBranch"`
}

// FromTripDetail maps a trip whose branches, stops and destinations are loaded.
func FromTripDetail(trip *model.Trip, role *enum.TripRole) TripDetailResponse {
	branches := make([]BranchResponse, 0, len(trip.Branches))
	for i := range trip.Branches {
		branches = append(branches, branchResponse(&trip.Branches[i]))
	}
	return TripDetailResponse{
		TripResponse: FromTrip(trip, role),
		Branches:     branches,
	}
}

func branchResponse(branch *model.TripBranch) BranchResponse {
	stops := make([]StopResponse, 0, len(branch.Stops))
	for i := range branch.Stops {
		stops = append(stops, stopResponse(&branch.Stops[i]))
	}
	return BranchResponse{
		ID:                     branch.ID,
		Label:                  branch.Label,
		SplitFromDestinationID: branch.SplitFromDestinationID,
		MergeToDestinationID:   branch.MergeToDestinationID,
		Stops:                  stops,
	}
}

func stopResponse(stop *model.BranchDestination) StopResponse {
	out := StopResponse{
		DestinationID: stop.DestinationID,
		OrderInBranch: stop.OrderInBranch,
	}
	if stop.Destination != nil {
		out.Name = stop.Destination.Name
		out.Lat = stop.Destination.Lat
		out.Lng = stop.Destination.Lng
	}
	return out
}

// AttachTravels sets branch.Travels[i] to the stored hop from stop i to stop i+1.
// A nil entry is a hop that has not been computed. Branches keep their stop list.
func AttachTravels(detail TripDetailResponse, graph *model.TravelGraph) TripDetailResponse {
	if graph == nil {
		return detail
	}
	for i := range detail.Branches {
		if i >= len(*graph) {
			break
		}
		row := (*graph)[i]
		if len(row) == 0 {
			continue
		}
		travels := make([]*TravelResponse, len(row))
		for j, travel := range row {
			if travel.ID == uuid.Nil {
				continue
			}
			copied := travelResponseFrom(travel)
			travels[j] = &copied
		}
		detail.Branches[i].Travels = travels
	}
	return detail
}
