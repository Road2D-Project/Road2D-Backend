package service

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func TestUpdateBranchMetadata(t *testing.T) {
	fix := newGraphFixture()
	description := "coast road"
	color := "#abc"
	svc := NewTripBranchService(fix.records, fix.records)

	got, err := svc.UpdateBranch(context.Background(), fix.tripID, fix.mainID, request.UpdateBranchRequest{
		Description: &description,
		Color:       &color,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.IsDraft || got.Description != "coast road" || got.Color != "#AABBCC" || got.ID != fix.mainID {
		t.Fatalf("metadata = %+v", got)
	}
	if len(fix.records.branchInfo) != 1 || fix.records.branchInfo[0].updates["color"] != "#AABBCC" {
		t.Fatalf("saved = %+v", fix.records.branchInfo)
	}
}

func TestUpdateBranchMetadataOnDraft(t *testing.T) {
	tripID := uuid.New()
	draftID := uuid.New()
	description := "not on the ride"
	color := "#00ff00"
	records := &graphRecords{
		withBranches: &model.Trip{
			Base:   utils.Base{ID: tripID},
			Status: enum.TripPlanning,
			Branches: []model.TripBranch{{
				Base:    utils.Base{ID: draftID},
				TripID:  tripID,
				Label:   model.DraftBranchLabel,
				IsDraft: true,
			}},
		},
	}
	got, err := NewTripBranchService(records, records).UpdateBranch(context.Background(), tripID, draftID, request.UpdateBranchRequest{
		Description: &description,
		Color:       &color,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsDraft || got.Label != model.DraftBranchLabel || got.Color != "#00FF00" || got.Description != description {
		t.Fatalf("draft metadata = %+v", got)
	}
	listed := response.DraftDestinations(&records.withBranches.Branches[0])
	if listed.ID != draftID || listed.Description != description || listed.Color != "#00FF00" {
		t.Fatalf("draft list = %+v", listed)
	}
}

func TestUpdateBranchMetadataClearsColor(t *testing.T) {
	fix := newGraphFixture()
	fix.records.withBranches.Branches[0].Color = "#112233"
	blank := "  "
	got, err := NewTripBranchService(fix.records, fix.records).UpdateBranch(context.Background(), fix.tripID, fix.mainID, request.UpdateBranchRequest{
		Color: &blank,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Color != "" || fix.records.branchInfo[0].updates["color"] != "" {
		t.Fatalf("color = %q, saved = %+v", got.Color, fix.records.branchInfo)
	}
}

func TestUpdateBranchMetadataRejectsBadColor(t *testing.T) {
	fix := newGraphFixture()
	color := "blue"
	_, err := NewTripBranchService(fix.records, fix.records).UpdateBranch(context.Background(), fix.tripID, fix.mainID, request.UpdateBranchRequest{
		Color: &color,
	})
	if !errors.Is(err, repository.ErrInvalidBranchColor) {
		t.Fatalf("err = %v", err)
	}
	if len(fix.records.branchInfo) != 0 {
		t.Fatal("invalid color was written")
	}
}

func TestUpdateBranchMetadataRequiresAField(t *testing.T) {
	fix := newGraphFixture()
	_, err := NewTripBranchService(fix.records, fix.records).UpdateBranch(context.Background(), fix.tripID, fix.mainID, request.UpdateBranchRequest{})
	if !errors.Is(err, repository.ErrNoBranchUpdate) {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateBranchMetadataRejectsCompletedTrip(t *testing.T) {
	fix := newGraphFixture()
	fix.records.withBranches.Status = enum.TripCompleted
	description := "late"
	_, err := NewTripBranchService(fix.records, fix.records).UpdateBranch(context.Background(), fix.tripID, fix.mainID, request.UpdateBranchRequest{
		Description: &description,
	})
	if !errors.Is(err, repository.ErrTripAlreadyCompleted) {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateBranchMetadataUnknownBranch(t *testing.T) {
	fix := newGraphFixture()
	description := "missing"
	_, err := NewTripBranchService(fix.records, fix.records).UpdateBranch(context.Background(), fix.tripID, uuid.New(), request.UpdateBranchRequest{
		Description: &description,
	})
	if !errors.Is(err, repository.ErrBranchNotFound) {
		t.Fatalf("err = %v", err)
	}
}
