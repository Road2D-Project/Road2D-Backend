package repository

import "errors"

var (
	ErrMemberRoleNotInCache = errors.New("member role not found in cache")
	ErrUserNotGroupMember   = errors.New("user is not an active group member")
	ErrGroupNotFound        = errors.New("group not found")
	ErrGroupHasTrips        = errors.New("group still has trips")
	ErrAdminUserNotFound    = errors.New("admin user not found")
	ErrNoGroupUpdate        = errors.New("no fields to update")
)
