package repository

import "errors"

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrUnavailable          = errors.New("chat temporarily unavailable")
	ErrMessageNotFound      = errors.New("message not found")
	ErrInvalidCursor        = errors.New("invalid event cursor")
)

type ReplayExpiredError struct {
	MinAfterEventSeq int64
	LatestEventSeq   int64
}

func (*ReplayExpiredError) Error() string { return "event cursor expired; synchronize conversation" }
