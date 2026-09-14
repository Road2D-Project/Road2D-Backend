package middleware

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/service"
	"Road-To-Destination-BE/module/share"
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthUserRepository interface {
	FindUserByUsername(ctx context.Context, username string) (*model.User, error)
}

type AuthenticationMiddleware struct {
	userRepo AuthUserRepository
}

func NewAuthenticationMiddleware(userRepo AuthUserRepository) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{userRepo: userRepo}
}

func (auth *AuthenticationMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Authorization header is empty"))
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Authorization header is invalid"))
			return
		}

		tokenString := parts[1]
		claims, err := service.VerifyAccessToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Invalid token"))
			return
		}
		username, ok := claims["username"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Invalid token payload"))
			return
		}

		authorizedUser, err := auth.userRepo.FindUserByUsername(c.Request.Context(), username)
		if err == nil && authorizedUser != nil {
			c.Set("currentUser", authorizedUser)
			c.Set("username", username)
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Unauthorized"))
	}
}
func GetCurrentUser(c *gin.Context) *model.User {
	user, exists := c.Get("currentUser")
	if !exists {
		return nil
	}
	return user.(*model.User)
}
func GetCurrentUsername(c *gin.Context) string {
	user := GetCurrentUser(c)
	if user != nil {
		return user.Username
	}
	return ""
}
