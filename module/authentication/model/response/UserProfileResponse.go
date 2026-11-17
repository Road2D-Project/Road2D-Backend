package response

import "Road-To-Destination-BE/module/authentication/model"

// TODO: user must set contract on their profile
type UserProfileResponse struct {
	Username    string  `json:"username" gorm:"column:username;type:varchar(255);unique;not null"`
	Email       string  `json:"email" gorm:"column:email;type:varchar(255);unique;not null" validate:"email"`
	NumberPhone string  `json:"numberPhone" gorm:"column:number_phone;type:varchar(255);not null" validate:"numeric"`
	AvatarURL   *string `json:"avatarUrl,omitempty" gorm:"column:avatar_url;type:text"`
}

func UserToUserProfileResponse(user *model.User, isOwner bool) UserProfileResponse {
	response := UserProfileResponse{
		Username:  user.Username,
		AvatarURL: user.AvatarURL,
	}
	if isOwner {
		response.NumberPhone = user.NumberPhone
		response.Email = user.Email
	}
	return response
}
