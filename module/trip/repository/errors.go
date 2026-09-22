package repository

import "errors"

var (
	ErrMemberRoleNotInCache  = errors.New("member role not found in cache")
	ErrUserNotTripMember     = errors.New("user is not an active trip member")
	ErrTripNotFound          = errors.New("trip not found")
	ErrGroupNotFound         = errors.New("group not found")
	ErrUserNotGroupMember    = errors.New("user is not an active group member")
	ErrNoTripUpdate          = errors.New("no fields to update")
	ErrAlreadyTripMember     = errors.New("user is already a trip member")
	ErrInviteNotFound        = errors.New("invite link not found")
	ErrNoSuccessorToTransfer = errors.New("no member to transfer leadership to")
	ErrInternalServerError   = errors.New("internal server error")
)
