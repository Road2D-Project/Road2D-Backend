package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

var (
	ErrTokenExpired     = errors.New("token is expired")
	ErrTokenInvalid     = errors.New("token is invalid")
	ErrRefreshRevoked   = errors.New("refresh token has been revoked")
	ErrSessionRevoked   = errors.New("user session has been revoked")
	ErrAuthUnavailable  = errors.New("authentication temporarily unavailable")
	ErrJWTConfiguration = errors.New("invalid JWT secret configuration")
)

func jwtSecret(name string) ([]byte, error) {
	secret := os.Getenv(name)
	if len(secret) < 32 || strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("%w: %s must contain at least 32 bytes", ErrJWTConfiguration, name)
	}
	return []byte(secret), nil
}

// ValidateJWTSecrets runs before the server connects to infrastructure or listens.
func ValidateJWTSecrets() error {
	names := []string{"SECRET_JWT", "SECRET_REFRESH_JWT", "SECRET_MAIL_JWT"}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		secret, err := jwtSecret(name)
		if err != nil {
			return err
		}
		if seen[string(secret)] {
			return fmt.Errorf("%w: access, refresh and reset secrets must be distinct", ErrJWTConfiguration)
		}
		seen[string(secret)] = true
	}
	return nil
}

func sessionClaims(user *model.User, tokenUse string, lifetime time.Duration) (jwt.MapClaims, error) {
	if user == nil || user.ID == uuid.Nil || user.Username == "" || user.SessionVersion < 1 {
		return nil, ErrTokenInvalid
	}
	return jwt.MapClaims{
		"authorized":     true,
		"sub":            user.ID.String(),
		"username":       user.Username,
		"sessionVersion": user.SessionVersion,
		"tokenUse":       tokenUse,
		"jti":            uuid.NewString(),
		"exp":            time.Now().Add(lifetime).Unix(),
	}, nil
}

func signClaims(claims jwt.MapClaims, secretName string) (string, error) {
	secret, err := jwtSecret(secretName)
	if err != nil {
		return "", err
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func CreateToken(user *model.User) (string, error) {
	claims, err := sessionClaims(user, "access", time.Hour)
	if err != nil {
		return "", err
	}
	return signClaims(claims, "SECRET_JWT")
}

func CreateRefreshToken(user *model.User) (string, error) {
	claims, err := sessionClaims(user, "refresh", 27*24*time.Hour)
	if err != nil {
		return "", err
	}
	return signClaims(claims, "SECRET_REFRESH_JWT")
}

func CreateResetPasswordToken(form ForgetPasswordForm) (string, error) {
	user := &model.User{Username: form.Username, SessionVersion: form.SessionVersion}
	user.ID = form.UserId
	claims, err := sessionClaims(user, "reset", 15*time.Minute)
	if err != nil {
		return "", err
	}
	claims["id"] = form.UserId.String()
	claims["email"] = form.Email
	return signClaims(claims, "SECRET_MAIL_JWT")
}

func verifyToken(raw, secretName, tokenUse string) (jwt.MapClaims, error) {
	secret, err := jwtSecret(secretName)
	if err != nil {
		return nil, err
	}
	parser := jwt.Parser{ValidMethods: []string{jwt.SigningMethodHS256.Alg()}, UseJSONNumber: true}
	token, err := parser.Parse(raw, func(*jwt.Token) (interface{}, error) { return secret, nil })
	if err != nil {
		var validation *jwt.ValidationError
		if errors.As(err, &validation) && validation.Errors == jwt.ValidationErrorExpired {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid || claims["tokenUse"] != tokenUse {
		return nil, ErrTokenInvalid
	}
	expiry, hasExpiry := ClaimsExpiry(claims)
	_, hasVersion := ClaimsSessionVersion(claims)
	subject, _ := claims["sub"].(string)
	id, idErr := uuid.Parse(subject)
	username, _ := claims["username"].(string)
	if !hasExpiry || !expiry.After(time.Now()) || !hasVersion || idErr != nil || id == uuid.Nil || username == "" {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func VerifyAccessToken(raw string) (jwt.MapClaims, error) {
	return verifyToken(raw, "SECRET_JWT", "access")
}

func VerifyRefreshToken(raw string) (jwt.MapClaims, error) {
	return verifyToken(raw, "SECRET_REFRESH_JWT", "refresh")
}

func VerifyResetPasswordToken(raw string) (jwt.MapClaims, error) {
	return verifyToken(raw, "SECRET_MAIL_JWT", "reset")
}

func ClaimsSessionVersion(claims jwt.MapClaims) (int64, bool) {
	raw, ok := claims["sessionVersion"].(json.Number)
	if !ok {
		return 0, false
	}
	version, err := raw.Int64()
	return version, err == nil && version > 0
}

func ValidateUserSession(claims jwt.MapClaims, user *model.User) error {
	version, ok := ClaimsSessionVersion(claims)
	if user == nil || !ok || claims["sub"] != user.ID.String() ||
		claims["username"] != user.Username || version != user.SessionVersion {
		return ErrSessionRevoked
	}
	return nil
}

func ClaimsExpiry(claims jwt.MapClaims) (time.Time, bool) {
	var seconds int64
	switch raw := claims["exp"].(type) {
	case json.Number:
		value, err := raw.Int64()
		if err != nil {
			return time.Time{}, false
		}
		seconds = value
	case float64:
		seconds = int64(raw)
	case int64:
		seconds = raw
	case int:
		seconds = int64(raw)
	default:
		return time.Time{}, false
	}
	return time.Unix(seconds, 0).UTC(), seconds > 0
}
