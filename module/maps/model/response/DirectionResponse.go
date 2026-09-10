package response

type DirectionResponse struct {
	GeocodedWaypoints []GeocodedWaypoint `json:"geocoded_waypoints"`
	Routes            []Route           `json:"routes"`
	Status            string             `json:"status"`
}

type GeocodedWaypoint struct {
	GeocoderStatus string `json:"geocoder_status"`
	PlaceID        string `json:"place_id"`
}

type Route struct {
	Legs              []Leg             `json:"legs"`
	OverviewPolyline  OverviewPolyline  `json:"overview_polyline"`
	Summary           string            `json:"summary"`
	Warnings          []string          `json:"warnings"`
	WaypointOrder     []int            `json:"waypoint_order"`
}

type Leg struct {
	Distance      TextValue `json:"distance"`
	Duration      TextValue `json:"duration"`
	StartAddress  string    `json:"start_address"`
	EndAddress    string    `json:"end_address"`
	StartLocation LatLng    `json:"start_location"`
	EndLocation   LatLng    `json:"end_location"`
	Steps         []Step    `json:"steps"`
}

type Step struct {
	Distance         TextValue        `json:"distance"`
	Duration         TextValue        `json:"duration"`
	StartLocation    LatLng           `json:"start_location"`
	EndLocation      LatLng           `json:"end_location"`
	HTMLInstructions string            `json:"html_instructions"`
	Maneuver         string            `json:"maneuver"`
	Polyline         OverviewPolyline  `json:"polyline"`
	TravelMode       string            `json:"travel_mode"`
}

type TextValue struct {
	Text  string `json:"text"`
	Value int    `json:"value"`
}

type OverviewPolyline struct {
	Points string `json:"points"`
}
