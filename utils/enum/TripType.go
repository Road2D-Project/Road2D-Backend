package enum

// TripType is the subscription tier stored on a trip. JSON and Postgres use the
// lowercase name: "bronze" | "silver" | "gold" | "diamond".
//
// The tier is chosen when the trip is created and is not editable later: the
// member cap is frozen from it, and changing either one is a subscription change.
// Only bronze has a cap today (15 seats, including the leader). The other names
// exist so a later policy can attach its own limit without a schema change.
//
//go:generate go tool enumer -type=TripType -json -text -sql -trimprefix=TripType -transform=lower
type TripType int

const (
	TripTypeBronze TripType = iota
	TripTypeSilver
	TripTypeGold
	TripTypeDiamond
)

const bronzeMemberLimit = 15

// MemberLimit reports the seat cap for this tier. The boolean is false when
// that tier has no policy yet, so callers must not invent a number.
func (t TripType) MemberLimit() (int, bool) {
	if t == TripTypeBronze {
		return bronzeMemberLimit, true
	}
	return 0, false
}
