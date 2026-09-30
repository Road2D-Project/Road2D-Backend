package model

// InitialMainBranch is the route written with a new trip.
// Destinations are pins forked from the reviewed locations, in stop order.
// Travels[i] is the hop from Destinations[i] to Destinations[i+1].
type InitialMainBranch struct {
	Destinations []Destination
	Travels      []Travel
}
