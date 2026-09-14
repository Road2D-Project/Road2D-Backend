package request

type ResetPasswordRequest struct {
	Password        string `json:"password" form:"password" validate:"strongPassword"`
	ConfirmPassword string `json:"confirm_password" form:"confirm_password" validate:"strongPassword"`
}
