package request

type UpdateMemberRequest struct {
	Nickname string `json:"nickname" binding:"required,min=1,max=64"`
}
