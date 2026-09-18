package request

type CreateGroupRequest struct {
	Name           string   `json:"name" binding:"required,min=1,max=255"`
	Description    string   `json:"description" binding:"max=4000"`
	AdminUsernames []string `json:"adminUsernames" binding:"omitempty,max=50,dive,min=1,max=32"`
}
