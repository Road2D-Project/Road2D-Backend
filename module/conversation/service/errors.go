package service

import "errors"

var ErrInvalidQuery = errors.New("invalid chat pagination")

var (
	ErrInvalidInput        = errors.New("invalid chat request")
	ErrForbidden           = errors.New("not allowed to modify this message")
	ErrConflict            = errors.New("message deleted, unsupported, or revision changed")
	ErrIdempotencyConflict = errors.New("clientMessageId already used with different content")
)
