package chatting

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

// PersonalCoordinator handles sync commands for one authenticated user's room.
type PersonalCoordinator struct {
	lastResync map[uuid.UUID]time.Time
}

func NewPersonalModule() realtime.ModuleFactory {
	return func(string) (realtime.Module, error) {
		return &PersonalCoordinator{lastResync: make(map[uuid.UUID]time.Time)}, nil
	}
}
