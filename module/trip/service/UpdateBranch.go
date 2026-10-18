package service

import (
	"context"
	"strings"

	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// UpdateBranch patches description and color on one branch.
// The draft inbox is included. Stop edits reject it because split and merge
// do not apply to loose pins; these two fields are only display metadata.
// A completed trip keeps the snapshot it already shared.
func (s *TripBranchService) UpdateBranch(ctx context.Context, tripID, branchID uuid.UUID, req request.UpdateBranchRequest) (*response.BranchMetadataResponse, error) {
	if req.Description == nil && req.Color == nil {
		return nil, repository.ErrNoBranchUpdate
	}
	stored, err := s.branches.FindTripWithBranches(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, repository.ErrTripNotFound
	}
	if stored.Status == enum.TripCompleted {
		return nil, repository.ErrTripAlreadyCompleted
	}
	idx := branchIndex(stored.Branches, branchID)
	if idx < 0 {
		return nil, repository.ErrBranchNotFound
	}
	branch := &stored.Branches[idx]
	updates := map[string]any{}
	if req.Description != nil {
		cleaned := utils.Santize(*req.Description)
		updates["description"] = cleaned
		branch.Description = cleaned
	}
	if req.Color != nil {
		color, err := normalizeHexColor(*req.Color)
		if err != nil {
			return nil, err
		}
		updates["color"] = color
		branch.Color = color
	}
	if err := s.branches.UpdateBranchInfo(ctx, tripID, branchID, updates); err != nil {
		return nil, err
	}
	out := response.BranchMetadata(branch)
	return &out, nil
}

// normalizeHexColor accepts #RGB and #RRGGBB and stores #RRGGBB.
// An empty value clears the color.
func normalizeHexColor(raw string) (string, error) {
	cleaned := strings.ToUpper(strings.TrimSpace(raw))
	if cleaned == "" {
		return "", nil
	}
	if (len(cleaned) != 4 && len(cleaned) != 7) || cleaned[0] != '#' || !hexDigits(cleaned[1:]) {
		return "", repository.ErrInvalidBranchColor
	}
	if len(cleaned) == 4 {
		return string([]byte{'#', cleaned[1], cleaned[1], cleaned[2], cleaned[2], cleaned[3], cleaned[3]}), nil
	}
	return cleaned, nil
}

func hexDigits(value string) bool {
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
