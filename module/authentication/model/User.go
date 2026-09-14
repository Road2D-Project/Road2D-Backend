package model

import (
	"Road-To-Destination-BE/utils"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	utils.Base
	Username string `json:"username" gorm:"column:username;type:varchar(255);unique;not null"`
	Email    string `json:"email" gorm:"column:email;type:varchar(255);unique;not null" validate:"email"`
	Password string `json:"-" gorm:"column:password;type:varchar(255);not null" swaggerignore:"true" validate:"strongPassword"`
}

func (User) TableName() string {
	return "users"
}

func (user *User) CheckPassword(password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return false, err
	}
	return true, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
