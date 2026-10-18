package request

// UpdateBranchRequest patches display metadata on one branch.
// Label stays as it is. The draft inbox accepts the same fields.
type UpdateBranchRequest struct {
	Description *string `json:"description" binding:"omitempty,max=2000"`
	Color       *string `json:"color" binding:"omitempty,max=32"`
}
