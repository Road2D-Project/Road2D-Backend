package enum

//go:generate go tool enumer -type=LegSource -json -text -sql -trimprefix=LegSource -transform=lower

type LegSource int

const (
	LegSourceDirection LegSource = iota
	LegSourceTrip
)
