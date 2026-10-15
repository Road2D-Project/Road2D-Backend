package request

type ConfirmAvatarRequest struct {
	Path string `json:"path" binding:"required"`
}
