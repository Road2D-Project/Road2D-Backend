package model

import (
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type travelKey struct {
	From uuid.UUID
	To   uuid.UUID
}

type travelRow struct {
	FromDestinationID uuid.UUID
	ToDestinationID   uuid.UUID
	Polyline          string
	DistanceM         float64
	DurationS         float64
}

func LoadTravels(db *gorm.DB, tripID uuid.UUID) (map[travelKey]travelRow, error) {
	var rows []travelRow
	err := db.Raw(`
		SELECT from_destination_id, to_destination_id, polyline, distance_m, duration_s
		FROM travels
		WHERE trip_id = ? AND vehicle = ?
	`, tripID, enum.BIKE.String()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[travelKey]travelRow, len(rows))
	for i := range rows {
		out[travelKey{rows[i].FromDestinationID, rows[i].ToDestinationID}] = rows[i]
	}
	return out, nil
}

func assembleHops(branch *TravelBranch, travels map[travelKey]travelRow) {
	if branch == nil || len(branch.Stops) < 2 {
		return
	}
	branch.Hops = make([]Hop, len(branch.Stops)-1)
	branch.TotalDistanceM = 0
	for i := 0; i < len(branch.Stops)-1; i++ {
		from := branch.Stops[i].DestinationID
		to := branch.Stops[i+1].DestinationID
		hop := Hop{FromDestinationID: from, ToDestinationID: to}
		row, ok := travels[travelKey{from, to}]
		if ok && row.Polyline != "" {
			points, err := DecodePolyline(row.Polyline)
			if err == nil && len(points) > 0 {
				hop.Points = points
				hop.DistanceM = row.DistanceM
				hop.DurationS = row.DurationS
				hop.Ready = true
			}
		}
		branch.Hops[i] = hop
		branch.TotalDistanceM += hop.DistanceM
	}
}
