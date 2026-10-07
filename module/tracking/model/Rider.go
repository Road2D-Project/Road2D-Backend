package model

import (
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// Các thông tin ban đầu, ít cập realtime ko tính vị trí
// 1 dẫn xuất từ trip member
// static data, only assign when load
type Rider struct {
	UserID           uuid.UUID
	Role             enum.TripRole
	Nickname         string
	AssignedBranchID uuid.UUID
}
