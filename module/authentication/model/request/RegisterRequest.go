package request

type RegisterRequest struct {
	Username        string `json:"username" binding:"required,min=4,max=16" gorm:"column:username;type:varchar(255)"`
	Email           string `json:"email" binding:"required,email" gorm:"column:email;type:varchar(255)"`
	Password        string `json:"password" binding:"required,min=8,max=32,eqfield=Password" gorm:"column:password;type:varchar(255)" validate:"strongPassword"`
	ConfirmPassword string `json:"confirmPassword" binding:"required,min=8,max=32,eqfield=Password" gorm:"column:password;type:varchar(255)"`
}
