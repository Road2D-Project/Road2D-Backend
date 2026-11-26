package realtime

import "time"

// RoomOptions applies to every room created by a hub. TickInterval zero disables
// OnTick and BroadcastLatest flushing; IdleTimeout must remain positive.
type RoomOptions struct {
	IdleTimeout       time.Duration
	TickInterval      time.Duration
	ControlBufferSize int
	MaxEventBytes     int
}

// DefaultRoomOptions preserves the tracking runtime's existing timing.
func DefaultRoomOptions() RoomOptions {
	return RoomOptions{
		IdleTimeout:       5 * time.Minute,
		TickInterval:      500 * time.Millisecond,
		ControlBufferSize: 64,
		MaxEventBytes:     16 * 1024,
	}
}

func (o RoomOptions) validate() error {
	if o.IdleTimeout <= 0 || o.TickInterval < 0 || o.ControlBufferSize <= 0 || o.MaxEventBytes <= 0 {
		return ErrInvalidRoomOptions
	}
	return nil
}
