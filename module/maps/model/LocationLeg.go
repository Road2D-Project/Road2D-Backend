package model

import "time"

// LocationLeg is the shared, mutable cache of a computed A→B route.
// Travel snapshots (frozen per trip) live in the routing domain, not here.
type LocationLeg struct {
	Origin      string    `json:"origin"`
	Destination string    `json:"destination"`
	Vehicle     string    `json:"vehicle"`
	DistanceM   int       `json:"distance_m"`
	DurationS   int       `json:"duration_s"`
	Polyline    string    `json:"polyline"`
	ComputedAt  time.Time `json:"computed_at"`
}
