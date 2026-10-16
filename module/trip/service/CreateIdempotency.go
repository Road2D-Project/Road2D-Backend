package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

const forkedTripIdempotencyHash = "fork"

// createFingerprint is the normalized create body. Field order is the hash input.
// Names and notes are already sanitized. Member ids are the seated roster, leader excluded.
// A main branch contributes catalog location ids and the reviewed hop, not the client pin names.
type createFingerprint struct {
	Name          string                   `json:"name"`
	Note          string                   `json:"note"`
	TripType      string                   `json:"tripType"`
	Visibility    bool                     `json:"visibility"`
	StartTime     string                   `json:"startTime"`
	EndTime       string                   `json:"endTime"`
	MemberUserIDs []string                 `json:"memberUserIds"`
	MainBranch    *createBranchFingerprint `json:"mainBranch,omitempty"`
}

type createBranchFingerprint struct {
	LocationIDs []string               `json:"locationIds"`
	Legs        []createLegFingerprint `json:"legs"`
}

type createLegFingerprint struct {
	FromLocationID string  `json:"fromLocationId"`
	ToLocationID   string  `json:"toLocationId"`
	Vehicle        string  `json:"vehicle"`
	Polyline       string  `json:"polyline"`
	DistanceM      float64 `json:"distanceM"`
	DurationS      float64 `json:"durationS"`
}

func parseIdempotencyKey(raw string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == uuid.Nil {
		return "", repository.ErrIdempotencyKeyRequired
	}
	return parsed.String(), nil
}

func createRequestHash(name, note string, tripType enum.TripType, visibility bool, start, end *time.Time, memberIDs []uuid.UUID, main *response.PreviewLocationsResponse) (string, error) {
	members := make([]string, len(memberIDs))
	for i, id := range memberIDs {
		members[i] = id.String()
	}
	body, err := json.Marshal(createFingerprint{
		Name:          name,
		Note:          note,
		TripType:      tripType.String(),
		Visibility:    visibility,
		StartTime:     formatFingerprintTime(start),
		EndTime:       formatFingerprintTime(end),
		MemberUserIDs: members,
		MainBranch:    branchFingerprint(main),
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func formatFingerprintTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func branchFingerprint(main *response.PreviewLocationsResponse) *createBranchFingerprint {
	if main == nil {
		return nil
	}
	locations := make([]string, len(main.Locations))
	for i, stop := range main.Locations {
		locations[i] = stop.LocationID.String()
	}
	legs := make([]createLegFingerprint, len(main.Legs))
	for i, leg := range main.Legs {
		legs[i] = createLegFingerprint{
			FromLocationID: locationIDString(leg.From.LocationID),
			ToLocationID:   locationIDString(leg.To.LocationID),
			Vehicle:        leg.Vehicle.String(),
			Polyline:       leg.Polyline,
			DistanceM:      leg.DistanceM,
			DurationS:      leg.DurationS,
		}
	}
	return &createBranchFingerprint{LocationIDs: locations, Legs: legs}
}

func locationIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
