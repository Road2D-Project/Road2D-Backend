package service

import (
	"testing"
	"time"

	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func TestCreateRequestHashIgnoresTimeZoneAndClientPinNames(t *testing.T) {
	member := uuid.New()
	place := uuid.New()
	other := uuid.New()
	local := time.Date(2026, 10, 16, 9, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	utc := local.UTC()
	named := &response.PreviewLocationsResponse{
		Locations: []response.PreviewLocation{
			{LocationID: place, Name: "client", Lat: 1},
			{LocationID: other, Name: "other", Lat: 2},
		},
		Legs: []response.ComputeBranchLeg{{
			From:      response.ComputeBranchStop{Name: "client", LocationID: &place},
			To:        response.ComputeBranchStop{Name: "other", LocationID: &other},
			Vehicle:   enum.BIKE,
			Polyline:  "abc",
			DistanceM: 10,
			DurationS: 4,
		}},
	}
	renamed := *named
	renamed.Locations = []response.PreviewLocation{
		{LocationID: place, Name: "catalog"},
		{LocationID: other, Name: "catalog"},
	}
	left, err := createRequestHash("Ha Giang", "note", enum.TripTypeBronze, false, &local, nil, []uuid.UUID{member}, named)
	if err != nil {
		t.Fatal(err)
	}
	right, err := createRequestHash("Ha Giang", "note", enum.TripTypeBronze, false, &utc, nil, []uuid.UUID{member}, &renamed)
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("hash %s vs %s", left, right)
	}
}

func TestCreateRequestHashChangesWhenMemberOrderChanges(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	left, err := createRequestHash("Ha Giang", "", enum.TripTypeBronze, false, nil, nil, []uuid.UUID{first, second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	right, err := createRequestHash("Ha Giang", "", enum.TripTypeBronze, false, nil, nil, []uuid.UUID{second, first}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if left == right {
		t.Fatal("member order should change the hash")
	}
}
