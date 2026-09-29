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
	ErrDestinationNotFound   = errors.New("destination not found")
	ErrDestinationNotEditing = errors.New("destination is not editing")
	ErrDestinationInUse      = errors.New("destination is still used by a trip that is not planning")
	ErrLocationNotFound      = errors.New("location not found")
	ErrInvalidTripGraph      = errors.New("invalid trip graph")
	ErrBranchNotFound        = errors.New("branch not found")
	ErrBranchStopNotFound    = errors.New("branch stop not found")
	ErrDraftBranchExcluded   = errors.New("draft branch is excluded from the route")
	ErrNoBranchStopUpdate    = errors.New("no fields to update on the branch stop")
)
