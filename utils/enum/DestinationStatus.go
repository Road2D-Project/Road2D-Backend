package enum

// DestinationStatus is the DoAn1 lifecycle of a trip pin.
//
//go:generate go tool enumer -type=DestinationStatus -json -text -sql

type DestinationStatus int

const (
	Editing DestinationStatus = iota
	Confirm
	Deleted
)
