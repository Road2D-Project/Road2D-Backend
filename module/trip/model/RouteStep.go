package model

type RouteStep struct {
	Instruction string  `json:"instruction"`
	Maneuver    string  `json:"maneuver,omitempty"`
	DistanceM   float64 `json:"distanceM"`
	DurationS   float64 `json:"durationS"`
	Polyline    string  `json:"polyline"`
	StartLat    float64 `json:"startLat"`
	StartLng    float64 `json:"startLng"`
	EndLat      float64 `json:"endLat"`
	EndLng      float64 `json:"endLng"`
	TravelMode  string  `json:"travelMode,omitempty"`
}
