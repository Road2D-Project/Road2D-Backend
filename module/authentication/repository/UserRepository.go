package repository

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/share/outbox"
	"context"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserRepository struct {
	db        *gorm.DB
	lifecycle UserAccessLifecycle
}

var ErrSessionChanged = errors.New("user session has changed")

func (u *UserRepository) ResetPassword(ctx context.Context, userId uuid.UUID, newPassword string, expectedVersion int64) error {
	user, err := u.FindUserByID(ctx, userId)
	if err != nil {
		return err
	}
	if expectedVersion < 1 || user.SessionVersion != expectedVersion {
		return ErrSessionChanged
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(newPassword))
	if err == nil {
		return errors.New("new password is same as old one ")
	}
	newHashPassword, err := model.HashPassword(newPassword)
	if err != nil {
		return err
	}
	change := func(ctx context.Context) error {
		return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&model.User{}).
				Where("id = ? AND session_version = ?", userId, expectedVersion).
				Updates(map[string]interface{}{
					"password":        newHashPassword,
					"session_version": gorm.Expr("session_version + 1"),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrSessionChanged
			}
			return outbox.AppendSessionRevoked(tx, userId, "", expectedVersion+1)
		})
	}
	if u.lifecycle != nil {
		return u.lifecycle.UserChange(ctx, userId, "", expectedVersion+1, change)
	}
	return change(ctx)
}

func (u *UserRepository) CheckExistedEmail(ctx context.Context, mail string) (*model.User, error) {
	var user model.User
	err := u.db.WithContext(ctx).Where("email = ?", mail).First(&user).Error
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
