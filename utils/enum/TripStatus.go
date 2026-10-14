package enum

//go:generate go tool enumer -type=TripStatus -json -text -sql -trimprefix=Trip -transform=lower

type TripStatus int

const (
	TripPlanning TripStatus = iota
	TripLocked
	// Trip is active when the leader want to start realtime trip
	TripActive
	// When all rider go to the final destination
	TripCompleted
	// Only tracking user position but not compute
	// Only can edit when leader give message
	TripStay
)
