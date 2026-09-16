package repository

import (
	"context"
	"errors"
	"time"

	"Road-To-Destination-BE/module/authentication/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type revokedRefreshPersist interface {
	Upsert(ctx context.Context, row *model.RevokedRefreshToken) error
	FindActive(ctx context.Context, hash string, now time.Time) (*model.RevokedRefreshToken, error)
	DeleteExpired(ctx context.Context, now time.Time) error
}

type gormRevokePersist struct {
	db *gorm.DB
}

func (p *gormRevokePersist) Upsert(ctx context.Context, row *model.RevokedRefreshToken) error {
	return p.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
}

func (p *gormRevokePersist) FindActive(ctx context.Context, hash string, now time.Time) (*model.RevokedRefreshToken, error) {
	var row model.RevokedRefreshToken
	err := p.db.WithContext(ctx).
		Where("token_hash = ? AND expires_at > ?", hash, now).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (p *gormRevokePersist) DeleteExpired(ctx context.Context, now time.Time) error {
	return p.db.WithContext(ctx).
		Where("expires_at <= ?", now).
		Delete(&model.RevokedRefreshToken{}).Error
}
