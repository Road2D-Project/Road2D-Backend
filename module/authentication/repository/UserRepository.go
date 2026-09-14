package repository

import (
	"Road-To-Destination-BE/module/authentication/model"
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func (u *UserRepository) ResetPassword(ctx context.Context, userId uuid.UUID, newPassword string) error {
	user, err := u.FindUserByID(ctx, userId)
	if err != nil {
		return err
	}
	newHashPassword, err := model.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if newHashPassword == user.Password {
		return errors.New("new password is same as old one ")
	}
	u.db.Table(model.User{}.TableName()).Where("id = ?", userId).Update("password", newHashPassword)
	return nil
}

func (u *UserRepository) CheckExistedEmail(ctx context.Context, mail string) (*model.User, error) {
	var user model.User
	err := u.db.Where("email = ?", mail).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Email does not exist ")
		} else {
			return nil, err
		}
	}
	return &user, nil
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (u *UserRepository) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := u.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *UserRepository) FindUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	if err := u.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *UserRepository) Register(username string, password string, email string, ctx context.Context) (uuid.UUID, error) {
	user := model.User{
		Username: username,
		Password: password,
		Email:    email,
	}
	if err := u.db.WithContext(ctx).Create(&user).Error; err != nil {
		return uuid.Nil, err
	}
	return user.ID, nil
}
