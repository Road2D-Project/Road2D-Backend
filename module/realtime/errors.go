package realtime

import "errors"

var (
	// ErrNotMember means the user is not allowed in this room.
	// The authorizer (trip membership today) returns it; the controller maps it to 403.
	ErrNotMember = errors.New("not a member of this room")

	// ErrRoomClosed means the room went idle between lookup and join, and the retry failed.
	ErrRoomClosed = errors.New("room closed")

	// ErrInvalidNamespace means a Mount name is empty, reserved, dotted, or already used.
	ErrInvalidNamespace = errors.New("invalid namespace")

	// ErrInvalidRoom means the room id is empty or longer than the hub accepts.
	ErrInvalidRoom = errors.New("invalid room id")
)
