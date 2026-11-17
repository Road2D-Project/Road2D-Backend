package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerRedactsTokensWithoutChangingRouteParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })
	router := gin.New()
	router.Use(LiftAccessToken(), RequestLogger())
	router.POST("/v1/auth/user/reset-password/:resetToken", func(c *gin.Context) {
		if c.Param("resetToken") != "sensitive-reset-token" {
			t.Fatal("reset token parameter changed")
		}
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost,
		"/v1/auth/user/reset-password/sensitive-reset-token?access_token=sensitive-access-token", nil))
	log := output.String()
	if strings.Contains(log, "sensitive-reset-token") || strings.Contains(log, "sensitive-access-token") || !strings.Contains(log, "[REDACTED]") {
		t.Fatalf("unexpected request log: %s", log)
	}
}
