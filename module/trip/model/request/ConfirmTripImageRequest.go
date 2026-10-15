package request

type ConfirmTripImageRequest struct {
	Path string `json:"path" binding:"required"`
}
