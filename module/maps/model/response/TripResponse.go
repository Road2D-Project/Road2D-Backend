package response

import "encoding/json"

// TripResponse is Goong Trip v2 (OSRM-shaped, not Google Directions).
type TripResponse struct {
	Code      string         `json:"code"`
	Trips     []GoongTrip    `json:"trips"`
	Waypoints []TripWaypoint `json:"waypoints"`
}

type GoongTrip struct {
	Distance   float64   `json:"distance"`
	Duration   float64   `json:"duration"`
	Geometry   string    `json:"geometry"`
	Legs       []TripLeg `json:"legs"`
	Weight     float64   `json:"weight"`
	WeightName string    `json:"weight_name"`
}

type TripLeg struct {
	Distance float64    `json:"distance"`
	Duration float64    `json:"duration"`
	Steps    []TripStep `json:"steps"`
	Summary  string     `json:"summary"`
	Weight   float64    `json:"weight"`
}

// TripStep is empty in the published sample; Goong may fill turn-by-turn later.
type TripStep struct {
	Distance         float64          `json:"distance"`
	Duration         float64          `json:"duration"`
	Weight           float64          `json:"weight"`
	Geometry         string           `json:"geometry"`
	Name             string           `json:"name"`
	Mode             string           `json:"mode"`
	Summary          string           `json:"summary"`
	HTMLInstructions string           `json:"html_instructions"`
	Maneuver         json.RawMessage  `json:"maneuver"`
	Polyline         OverviewPolyline `json:"polyline"`
}

type TripWaypoint struct {
	Distance      float64   `json:"distance"`
	Location      []float64 `json:"location"`
	PlaceID       string    `json:"place_id"`
	TripsIndex    int       `json:"trips_index"`
	WaypointIndex int       `json:"waypoint_index"`
}
