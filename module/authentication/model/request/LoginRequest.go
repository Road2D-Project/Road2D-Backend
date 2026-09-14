package request

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=4,max=16" gorm:"column:username;type:varchar(255)"`
	Password string `json:"password" binding:"required,min=8,max=32,eqfield=Password" gorm:"column:password;type:varchar(255)" validate:"strongPassword"`
}
