package repository

import (
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// FriendRow is another user on a friendship the caller can see.
type FriendRow struct {
	UserID      uuid.UUID
	Username    string
	Status      enum.FriendStatus
	RequesterID uuid.UUID
}

// UserCandidate is a search hit before ranking.
// PhoneMatched is true only when number_phone equals the full query.
// The phone number itself is not carried out of the repository.
type UserCandidate struct {
	UserID       uuid.UUID
	Username     string
	PhoneMatched bool
}
