package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type authSessionRepository struct {
	user *model.User
	err  error
}

func (r *authSessionRepository) FindUserByUsername(context.Context, string) (*model.User, error) {
	return r.user, r.err
}

func (*authSessionRepository) IsLoginSessionRevoked(context.Context, string) (bool, error) {
	return false, nil
}

func TestAuthenticationRejectsAccessAfterSessionVersionChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SECRET_JWT", "access-test-secret-01234567890123456789")
	user := &model.User{Username: "traveler", SessionVersion: 1}
	user.ID = uuid.New()
	token, err := service.CreateToken(user)
	if err != nil {
		t.Fatal(err)
	}
	repo := &authSessionRepository{user: user}
	router := gin.New()
	router.GET("/me", NewAuthenticationMiddleware(repo).RequireAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := func(want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != want {
			t.Fatalf("status = %d, want %d", recorder.Code, want)
		}
	}
	request(http.StatusNoContent)
	user.SessionVersion++
	request(http.StatusUnauthorized)
	user.SessionVersion = 1
	user.ID = uuid.New()
	request(http.StatusUnauthorized)
	repo.err = errors.New("database unavailable")
	request(http.StatusServiceUnavailable)
}
