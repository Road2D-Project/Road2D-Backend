package enum

//go:generate go tool enumer -type=TripStatus -json -text -sql -trimprefix=Trip -transform=lower

type TripStatus int

const (
	TripPlanning TripStatus = iota
	TripLocked
)
