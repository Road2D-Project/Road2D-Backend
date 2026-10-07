package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type branchRow struct {
	ID        uuid.UUID
	Label     string
	SplitFrom *uuid.UUID `gorm:"column:split_from_destination_id"`
	MergeTo   *uuid.UUID `gorm:"column:merge_to_destination_id"`
}

type stopRow struct {
	TripBranchID  uuid.UUID
	DestinationID uuid.UUID
	Name          string
	Lat           float64
	Lng           float64
	OrderInBranch int
}

func LoadBranches(db *gorm.DB, out *TripInformation) error {
	var branches []branchRow
	err := db.Raw(`
		SELECT id, label, split_from_destination_id, merge_to_destination_id
		FROM trip_branches
		WHERE trip_id = ? AND is_draft = false
	`, out.TripId).Scan(&branches).Error
	if err != nil {
		return err
	}
	for i := range branches {
		out.Branches[branches[i].ID] = &TravelBranch{
			ID:        branches[i].ID,
			Label:     branches[i].Label,
			SplitFrom: branches[i].SplitFrom,
			MergeTo:   branches[i].MergeTo,
		}
	}

	var stops []stopRow
	err = db.Raw(`
		SELECT bd.trip_branch_id, bd.destination_id, bd.order_in_branch,
		       d.name, d.lat, d.lng
		FROM branch_destinations AS bd
		JOIN destinations AS d ON d.id = bd.destination_id
		JOIN trip_branches AS tb ON tb.id = bd.trip_branch_id
		WHERE tb.trip_id = ? AND tb.is_draft = false
		ORDER BY bd.trip_branch_id, bd.order_in_branch
	`, out.TripId).Scan(&stops).Error
	if err != nil {
		return err
	}
	for i := range stops {
		branch := out.Branches[stops[i].TripBranchID]
		if branch == nil {
			continue
		}
		branch.Stops = append(branch.Stops, Stop{
			DestinationID: stops[i].DestinationID,
			Name:          stops[i].Name,
			Latitude:      stops[i].Lat,
			Longitude:     stops[i].Lng,
			Order:         stops[i].OrderInBranch,
		})
	}
	return nil
}
