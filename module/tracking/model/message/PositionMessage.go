package message

// PositionMessage is one GPS sample from a phone.
// BatteryPercent is 0–100. SignalDbm is the radio reading; the server turns it into bars.
type PositionMessage struct {
	Lat, Lng       float64
	BatteryPercent int
	SignalDbm      int
}

func (p PositionMessage) GetType() TrackingMsgType {
	return Position
}

// Battery clamps a percent into 0–100.
func (p PositionMessage) Battery() int {
	if p.BatteryPercent < 0 {
		return 0
	}
	if p.BatteryPercent > 100 {
		return 100
	}
	return p.BatteryPercent
}

// SignalBars maps a dBm reading to 0–5 bars.
func (p PositionMessage) SignalBars() int {
	switch {
	case p.SignalDbm >= -70:
		return 5
	case p.SignalDbm >= -85:
		return 4
	case p.SignalDbm >= -100:
		return 3
	case p.SignalDbm >= -110:
		return 2
	case p.SignalDbm >= -120:
		return 1
	default:
		return 0
	}
}
