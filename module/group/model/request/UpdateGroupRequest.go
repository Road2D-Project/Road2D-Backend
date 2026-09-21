package request

type UpdateGroupRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=255"`
	Description *string `json:"description" binding:"omitempty,max=4000"`
	Policy      *string `json:"policy" binding:"omitempty,max=4000"`
}
