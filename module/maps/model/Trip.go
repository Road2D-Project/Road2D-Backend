package model

import (
	"time"

	"Road-To-Destination-BE/module/utils/enum"
)

// Trip is the optimized multi-stop itinerary from Goong Trip v2.
type Trip struct {
	Origin      string       `json:"origin,omitempty"`
	Destination string       `json:"destination,omitempty"`
	Vehicle     enum.Vehicle `json:"vehicle" swaggertype:"string" example:"car"`
	Roundtrip   bool         `json:"roundtrip"`
	Code        string       `json:"code"`
	Trips       []TripRoute  `json:"trips"`
	Waypoints   []TripStop   `json:"waypoints"`
	VisitOrder  []TripStop   `json:"visit_order"`
	ComputedAt  time.Time    `json:"computed_at"`
}

// TripRoute is one optimized tour (usually a single element unless Goong returns several).
type TripRoute struct {
	Distance   float64    `json:"distance"`
	Duration   float64   `json:"duration"`
	Geometry   string     `json:"geometry"`
	Weight     float64   `json:"weight"`
	WeightName string     `json:"weight_name"`
	Legs       []TripLeg  `json:"legs"`
}

// TripLeg is the segment between two consecutive stops (origin→waypoint or waypoint→destination).
type TripLeg struct {
	Distance float64    `json:"distance"`
	Duration float64    `json:"duration"`
	Weight   float64    `json:"weight"`
	Summary  string      `json:"summary"`
	Steps    []TripStep `json:"steps"`
}

type TripStep struct {
	Distance    float64 `json:"distance"`
	Duration    float64 `json:"duration"`
	Weight      float64 `json:"weight,omitempty"`
	Summary     string  `json:"summary,omitempty"`
	Name        string  `json:"name,omitempty"`
	Instruction string  `json:"instruction,omitempty"`
	Maneuver    string  `json:"maneuver,omitempty"`
	Geometry    string  `json:"geometry,omitempty"`
}

// TripStop is a requested point snapped onto the road network.
type TripStop struct {
	InputIndex    int     `json:"input_index"`
	Distance      float64 `json:"distance"`
	Location      LatLng  `json:"location"`
	PlaceID       string  `json:"place_id"`
	TripsIndex    int     `json:"trips_index"`
	WaypointIndex int     `json:"waypoint_index"`
}
