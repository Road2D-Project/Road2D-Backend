package request

type ForgetPasswordRequest struct {
	Email string `json:"mail" binding:"required,email"`
}
