package model

import (
	"time"

	"Road-To-Destination-BE/module/utils/enum"
)

// LocationLeg is the shared, mutable cache of a computed A→B route.
// Travel snapshots (frozen per trip) live in the routing domain, not here.
type LocationLeg struct {
	Origin      string       `json:"origin"`
	Destination string       `json:"destination"`
	Vehicle     enum.Vehicle `json:"vehicle" swaggertype:"string" example:"bike"`
	DistanceM   int          `json:"distance_m"`
	DurationS   int          `json:"duration_s"`
	Polyline    string       `json:"polyline"`
	Steps       []RouteStep  `json:"steps"`
	ComputedAt  time.Time    `json:"computed_at"`
}

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type RouteStep struct {
	Instruction string `json:"instruction"`
	Maneuver    string `json:"maneuver,omitempty"`
	DistanceM   int    `json:"distance_m"`
	DurationS   int    `json:"duration_s"`
	Polyline    string `json:"polyline"`
	Start       LatLng `json:"start"`
	End         LatLng `json:"end"`
	TravelMode  string `json:"travel_mode,omitempty"`
}
