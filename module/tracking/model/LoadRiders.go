package model

import (
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type riderRow struct {
	UserID           uuid.UUID
	Role             enum.TripRole
	Nickname         string
	AssignedBranchID *uuid.UUID
}

func LoadRiders(db *gorm.DB, out *TripInformation) error {
	var rows []riderRow
	err := db.Raw(`
		SELECT user_id, role, nickname, assigned_branch_id
		FROM trip_members
		WHERE trip_id = ? AND status = ?
	`, out.TripId, enum.MembershipActive.String()).Scan(&rows).Error
	if err != nil {
		return err
	}
	for i := range rows {
		rider := &Rider{
			UserID:   rows[i].UserID,
			Role:     rows[i].Role,
			Nickname: rows[i].Nickname,
		}
		if rows[i].AssignedBranchID != nil {
			rider.AssignedBranchID = *rows[i].AssignedBranchID
		}
		out.Riders[rider.UserID] = rider
	}
	return nil
}
