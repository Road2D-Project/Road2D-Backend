package enum

// TripRole is the per-trip membership role, not a global account permission.
// JSON and Postgres store the lowercase name: "leader" | "member" | "admin".
//
// leader — Unique owner of the trip. May edit name, note, times, and visibility,
// mint the invite link, approve join requests, kick members, appoint or demote
// admins, delete the trip, or transfer leadership. Leaving moves leadership to
// the earliest-joined admin, else the earliest-joined member. If nobody remains,
// delete the trip.
// admin — May edit name, note, times, and visibility, mint the invite link,
// approve join requests, and kick members. Cannot delete the trip, kick the
// leader or another admin, or change roles.
// member — May read the trip, leave, invite a user (that invite stays pending
// until a leader or admin approves), or request to join via the invite link.
// Cannot edit the trip or approve members.
//
//go:generate go tool enumer -type=TripRole -json -text -sql -trimprefix=TripRole -transform=lower
type TripRole int

const (
	TripRoleLeader TripRole = iota
	TripRoleMember
	TripRoleAdmin
)

func (role TripRole) IsLeader() bool {
	return role == TripRoleLeader
}

func (role TripRole) IsAdmin() bool {
	return role == TripRoleAdmin
}

func (role TripRole) CanManageMembers() bool {
	return role == TripRoleLeader || role == TripRoleAdmin
}

func (role TripRole) CanEditTrip() bool {
	return role.CanManageMembers()
}

func (role TripRole) CanDeleteTrip() bool {
	return role == TripRoleLeader
}

func (role TripRole) CanMintInviteLink() bool {
	return role.CanManageMembers()
}

func (role TripRole) CanChangeRoles() bool {
	return role == TripRoleLeader
}
