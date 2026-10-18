package message

import (
	"time"

	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// Outbound wire types. Each one is sent with Session.Broadcast because the
// client must not skip a state edge.
const (
	OutPositions  = "tracking.positions"
	OutOffRoute   = "tracking.offRoute"
	OutLife       = "tracking.life"
	OutRedirect   = "tracking.redirect"
	OutConnection = "tracking.connection"
	OutStayMode   = "tracking.stayMode"
	OutSuccess    = "tracking.success"
	OutOnLeave    = "tracking.onLeave"
)

// RiderPosition is one rider in the tracking.positions batch.
type RiderPosition struct {
	UserID           uuid.UUID `json:"userId"`
	Lat              float64   `json:"lat"`
	Lng              float64   `json:"lng"`
	BatteryPercent   int       `json:"batteryPercent"`
	SignalBars       int       `json:"signalBars"`
	DriveStatus      enum.DriveStatus      `json:"driveStatus"`
	ConnectionStatus enum.ConnectionStatus `json:"connectionStatus"`
}

// OffRoute is sent each time the deviated flag of one rider flips.
type OffRoute struct {
	UserID    uuid.UUID `json:"userId"`
	Deviated  bool      `json:"deviated"`
	DistanceM float64   `json:"distanceM"`
	BranchID  uuid.UUID `json:"branchId"`
	HopIndex  int       `json:"hopIndex"`
	Lives     int       `json:"lives"`
}

// Life is sent only when the life count goes down.
type Life struct {
	UserID      uuid.UUID `json:"userId"`
	Lives       int       `json:"lives"`
	ImmuneUntil time.Time `json:"immuneUntil"`
	Exhausted   bool      `json:"exhausted"`
}

// Redirect status values.
const (
	RedirectPending = "pending"
	RedirectReady   = "ready"
	RedirectFailed  = "failed"
	RedirectCleared = "cleared"
)

// Redirect reason values.
const (
	RedirectReasonConfirmed     = "confirmed"
	RedirectReasonRefresh       = "refresh"
	RedirectReasonRejoined      = "rejoined"
	RedirectReasonArrivedStop   = "arrivedStop"
	RedirectReasonHopNotReady   = "hopNotReady"
	RedirectReasonProviderError = "providerError"
)

// RedirectPoint is a lat/lng pair on the wire.
type RedirectPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// RedirectTarget is the stop the temporary route leads to.
type RedirectTarget struct {
	DestinationID uuid.UUID `json:"destinationId"`
	Name          string    `json:"name"`
	Lat           float64   `json:"lat"`
	Lng           float64   `json:"lng"`
}

// Redirect follows the life of one temporary route.
// Polyline, DistanceM, and DurationS are set only when Status is ready.
type Redirect struct {
	UserID    uuid.UUID       `json:"userId"`
	Status    string          `json:"status"`
	Reason    string          `json:"reason"`
	Origin    RedirectPoint   `json:"origin"`
	Target    *RedirectTarget `json:"target,omitempty"`
	Polyline  string          `json:"polyline,omitempty"`
	DistanceM float64         `json:"distanceM,omitempty"`
	DurationS float64         `json:"durationS,omitempty"`
}
