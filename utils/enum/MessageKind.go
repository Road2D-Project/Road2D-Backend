package enum

// MessageKind stores lowercase names in JSON and PostgreSQL.
//
//go:generate go tool enumer -type=MessageKind -json -text -sql -trimprefix=MessageKind -transform=lower
type MessageKind int

const (
	MessageKindText MessageKind = iota
	MessageKindMedia
	MessageKindPoll
	MessageKindSystem
)
