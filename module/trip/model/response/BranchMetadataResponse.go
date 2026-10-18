package response

import (
	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
)

// BranchMetadataResponse is the display fields of one branch, including the draft inbox.
type BranchMetadataResponse struct {
	ID          uuid.UUID `json:"id" swaggertype:"string" format:"uuid"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	IsDraft     bool      `json:"isDraft"`
}

func BranchMetadata(branch *model.TripBranch) BranchMetadataResponse {
	return BranchMetadataResponse{
		ID:          branch.ID,
		Label:       branch.Label,
		Description: branch.Description,
		Color:       branch.Color,
		IsDraft:     branch.IsDraft,
	}
}
