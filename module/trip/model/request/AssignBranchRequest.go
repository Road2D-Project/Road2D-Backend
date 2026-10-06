package request

import "github.com/google/uuid"

type AssignBranchRequest struct {
	UserID   *uuid.UUID `json:"userId" binding:"required"`
	BranchID *uuid.UUID `json:"branchId" binding:"required"`
}
