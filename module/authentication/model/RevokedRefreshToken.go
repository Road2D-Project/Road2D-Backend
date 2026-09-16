package model

import "time"

// RevokedRefreshToken is the durable denylist row for a logged-out refresh JWT.
// token_hash is SHA-256 of the raw token (never store the JWT itself).
// The row can be deleted once expires_at has passed — the JWT is dead anyway.
type RevokedRefreshToken struct {
	TokenHash string    `json:"-" gorm:"column:token_hash;type:varchar(64);primaryKey"`
	Username  string    `json:"username,omitempty" gorm:"column:username;type:varchar(255);index"`
	ExpiresAt time.Time `json:"expiresAt" gorm:"column:expires_at;index;not null"`
	CreatedAt time.Time `json:"createdTime" gorm:"column:created_at;autoCreateTime"`
}

func (RevokedRefreshToken) TableName() string {
	return "revoked_refresh_tokens"
}
