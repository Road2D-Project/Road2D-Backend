package service

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var (
	ErrTokenExpired   = errors.New("token is expired")
	ErrTokenInvalid   = errors.New("token is invalid")
	ErrRefreshRevoked = errors.New("refresh token has been revoked")
)

func CreateToken(username string) (string, error) {
	claims := jwt.MapClaims{}
	claims["authorized"] = true
	claims["username"] = username
	claims["exp"] = time.Now().Add(time.Hour * 1).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("SECRET_JWT")))
}
func CreateRefreshToken(username string) (string, error) {
	claims := jwt.MapClaims{}
	claims["authorized"] = true
	claims["username"] = username
	claims["exp"] = time.Now().Add(time.Hour * 24 * 27).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("SECRET_REFRESH_JWT")))
}

// Nếu token hết exp -> parse tự động lỗi
func VerifyRefreshToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("SECRET_REFRESH_JWT")), nil // Sử dụng Refresh Key bí mật
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, ErrTokenExpired
			} else if ve.Errors&jwt.ValidationErrorClaimsInvalid != 0 {
				return nil, ErrTokenInvalid
			}

		}
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
func VerifyAccessToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("SECRET_JWT")), nil
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, ErrTokenExpired
			}
		}
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
func CreateResetPasswordToken(form ForgetPasswordForm) (string, error) {
	claims := jwt.MapClaims{}
	claims["authorized"] = true
	claims["username"] = form.Username
	claims["exp"] = time.Now().Add(time.Minute * 15).Unix()
	claims["email"] = form.Email
	claims["id"] = form.UserId.String()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("SECRET_MAIL_JWT")))
}
func VerifyResetPasswordToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("SECRET_MAIL_JWT")), nil
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, ErrTokenExpired
			} else if ve.Errors&jwt.ValidationErrorClaimsInvalid != 0 {
				return nil, ErrTokenInvalid
			}

		}
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func ClaimsExpiry(claims jwt.MapClaims) (time.Time, bool) {
	raw, ok := claims["exp"]
	if !ok || raw == nil {
		return time.Time{}, false
	}
	switch v := raw.(type) {
	case float64:
		return time.Unix(int64(v), 0).UTC(), true
	case int64:
		return time.Unix(v, 0).UTC(), true
	case int:
		return time.Unix(int64(v), 0).UTC(), true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(n, 0).UTC(), true
	default:
		return time.Time{}, false
	}
}
