package enum

// FriendStatus is the lifecycle of one friendship row.
// JSON and Postgres store the lowercase name: "pending" | "accepted" | "rejected".
//
// The pair (user_low_id, user_high_id) is unique. A later request reuses that row.
//
// pending  — One user sent a request; the other has not answered.
// accepted — The two users are friends.
// rejected — The addressee declined. Either user may send again, which
//            moves the same row back to pending.
//
//go:generate go tool enumer -type=FriendStatus -json -text -sql -trimprefix=FriendStatus -transform=lower

type FriendStatus int

const (
	FriendStatusPending FriendStatus = iota
	FriendStatusAccepted
	FriendStatusRejected
)

func (status FriendStatus) IsPending() bool {
	return status == FriendStatusPending
}

func (status FriendStatus) IsAccepted() bool {
	return status == FriendStatusAccepted
}

func (status FriendStatus) IsRejected() bool {
	return status == FriendStatusRejected
}
