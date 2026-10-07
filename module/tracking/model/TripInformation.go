package model

import (
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// xác định các thông tin cần được rút gọn
type TripInformation struct {
	TripId       uuid.UUID
	TripName     string
	Status       enum.TripStatus
	MainBranchID uuid.UUID
	Riders       map[uuid.UUID]*Rider
	Branches     map[uuid.UUID]*TravelBranch
}
