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

	// ErrModuleUnavailable means a mounted module failed to build, so the room
	// was not created. The socket is already upgraded; the lobby closes it.
	ErrModuleUnavailable = errors.New("realtime module unavailable")

	ErrInvalidRoomOptions       = errors.New("invalid realtime room options")
	ErrInvalidUser              = errors.New("invalid realtime user id")
	ErrInvalidServerEvent       = errors.New("invalid realtime server event")
	ErrEventTooLarge            = errors.New("realtime server event too large")
	ErrRoomQueueFull            = errors.New("realtime room control queue full")
	ErrInvalidConnectionOptions = errors.New("invalid realtime connection options")
	ErrConnectionExpired        = errors.New("realtime connection expired")
)
