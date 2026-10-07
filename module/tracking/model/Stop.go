package model

import "github.com/google/uuid"

type Stop struct {
	DestinationID uuid.UUID
	Name          string
	Latitude      float64
	Longitude     float64
	Order         int
}
type Point struct {
	Lat float64
	Lng float64
}
