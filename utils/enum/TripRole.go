package enum

// TripRole is the per-trip membership role, not a global account permission.
// JSON and Postgres store the lowercase name: "leader" | "member".
//
// leader — Unique owner of the trip. May edit name/note/times, mint the invite
//          link, delete the trip, or transfer leadership. Leaving auto-transfers
//          to the earliest-joined member. If nobody remains, delete the trip.
// member — May read the trip, leave, or join via invite link. Cannot edit the
//          trip or rotate the invite token.
//
//go:generate go tool enumer -type=TripRole -json -text -sql -trimprefix=TripRole -transform=lower

type TripRole int

const (
	TripRoleLeader TripRole = iota
	TripRoleMember
)

func (role TripRole) IsLeader() bool {
	return role == TripRoleLeader
}

func (role TripRole) CanEditTrip() bool {
	return role == TripRoleLeader
}

func (role TripRole) CanDeleteTrip() bool {
	return role == TripRoleLeader
}

func (role TripRole) CanMintInviteLink() bool {
	return role == TripRoleLeader
}
