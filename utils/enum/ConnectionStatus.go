package enum

//go:generate go tool enumer -type=ConnectionStatus -json -text -sql

type ConnectionStatus int

const (
	CONNECTING ConnectionStatus = iota
	UNSTABLE
	DISCONNECTED
	RECONNECTING
)
