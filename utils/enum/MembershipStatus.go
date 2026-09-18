package enum

// MembershipStatus is the lifecycle of one (group_id, user_id) row.
// JSON and Postgres store the lowercase name:
// "invited" | "pending" | "active" | "rejected" | "left" | "kicked".
//
// The pair (group_id, user_id) is unique. A later join reuses the same row
// instead of inserting a duplicate.
//
// invited  — An admin/owner invited this user; waiting for accept or decline.
// pending  — The user requested to join; waiting for admin/owner approval.
// active   — Current member. May see the roster, edit nickname, and leave.
// rejected — The join request was denied by an admin/owner.
// left     — The user left the group on their own.
// kicked   — An admin/owner removed the user.
//
// left, rejected, and kicked may re-enter via a new join request (pending) or
// a direct add/invite from staff.
//
//go:generate go tool enumer -type=MembershipStatus -json -text -sql -trimprefix=Membership -transform=lower

type MembershipStatus int

const (
	MembershipInvited MembershipStatus = iota
	MembershipPending
	MembershipActive
	MembershipRejected
	MembershipLeft
	MembershipKicked
)

func (status MembershipStatus) IsActive() bool {
	return status == MembershipActive
}

func (status MembershipStatus) IsInvited() bool {
	return status == MembershipInvited
}

func (status MembershipStatus) IsPending() bool {
	return status == MembershipPending
}

// CanRejoin is true when the same (group, user) row may be reused for a new join request.
func (status MembershipStatus) CanRejoin() bool {
	switch status {
	case MembershipLeft, MembershipRejected, MembershipKicked:
		return true
	default:
		return false
	}
}
