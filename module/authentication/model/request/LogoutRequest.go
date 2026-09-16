package request

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}
