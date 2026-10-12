package repository

import "errors"

var (
	ErrCannotFriendSelf    = errors.New("cannot send a friend request to yourself")
	ErrAlreadyFriends      = errors.New("users are already friends")
	ErrRequestAlreadySent  = errors.New("friend request already sent")
	ErrRequestNotFound     = errors.New("friend request not found")
	ErrNotRequestAddressee = errors.New("only the invited user can answer this request")
	ErrNotFriends          = errors.New("users are not friends")
	ErrUserNotFound        = errors.New("user not found")
	ErrEmptySearch         = errors.New("search query is required")
	ErrSearchQueryTooLong  = errors.New("search query is too long")
)
