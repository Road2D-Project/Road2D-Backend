package repository

import "errors"

var (
	ErrMemberRoleNotInCache = errors.New("member role not found in cache")
	ErrUserNotGroupMember   = errors.New("user is not an active group member")
	ErrGroupNotFound        = errors.New("group not found")
	ErrGroupHasTrips        = errors.New("group still has trips")
	ErrAdminUserNotFound    = errors.New("admin user not found")
	ErrNoGroupUpdate        = errors.New("no fields to update")
	ErrInternalServerError  = errors.New("internal server error")
	ErrUserNotFound         = errors.New("user not found")
	ErrCannotInviteSelf     = errors.New("cannot invite yourself")
	ErrAlreadyGroupMember   = errors.New("user is already a group member")
	ErrAlreadyInvited       = errors.New("user is already invited to this group")
	ErrJoinRequestPending   = errors.New("user already has a pending join request")
	ErrInvitationNotFound   = errors.New("invitation not found")
	ErrInvitationNotPending = errors.New("membership is not a pending invitation")
	ErrNotEnoughGroupMember = errors.New("At least 2 group member (include creator) are needed to join")
)
