package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/objectStorage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func avatarTestController(t *testing.T) AuthenticationController {
	t.Helper()
	t.Setenv("R2_ACCOUNT_ID", "test-account")
	t.Setenv("R2_BUCKET_NAME", "media")
	t.Setenv("R2_ACCESS_KEY_ID", "test-access")
	t.Setenv("R2_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("R2_ENDPOINT", "")
	t.Setenv("MEDIA_MAX_IMAGE_BYTES", "5000000")
	return AuthenticationController{storage: objectStorage.NewClient()}
}

func TestAvatarUploadRequiresSizeAndReturnsSignedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := avatarTestController(t)
	user := &model.User{}
	user.ID = uuid.New()
	router := gin.New()
	router.POST("/avatar", func(c *gin.Context) { c.Set("currentUser", user) }, ctrl.HandleRequestAvatar())
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"contentType=image/png", 400},
		{"contentType=image/png&sizeBytes=0", 400},
		{"contentType=image/png&sizeBytes=-1", 400},
		{"contentType=image/png&sizeBytes=invalid", 400},
		{"contentType=image/png&sizeBytes=5000001", 413},
		{"contentType=video/mp4&sizeBytes=123", 400},
		{"contentType=image/png&sizeBytes=123", 200},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest("POST", "/avatar?"+tc.query, nil))
		if recorder.Code != tc.status {
			t.Fatalf("%s: status=%d body=%s", tc.query, recorder.Code, recorder.Body.String())
		}
		if tc.status == http.StatusOK {
			var signed objectStorage.SignedUpload
			if err := json.Unmarshal(recorder.Body.Bytes(), &signed); err != nil {
				t.Fatal(err)
			}
			if signed.Headers["Content-Type"] != "image/png" || signed.Headers["Content-Length"] != "123" || signed.Method != "PUT" {
				t.Fatalf("unexpected signed response %+v", signed)
			}
			if err := objectStorage.ValidateOwnedPath(user.ID.String(), signed.Path); err != nil {
				t.Fatal(err)
			}
			if recorder.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("signed response is cacheable")
			}
		}
	}
}

func TestMyProfileSignsAvatarWithoutMutatingAuthenticatedRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := avatarTestController(t)
	user := &model.User{Username: "traveler"}
	user.ID = uuid.New()
	key := user.ID.String() + "/" + uuid.NewString() + ".png"
	user.AvatarPath = &key
	router := gin.New()
	router.GET("/me", func(c *gin.Context) { c.Set("currentUser", user) }, ctrl.HandleMyProfile())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/me", nil))
	if recorder.Code != 200 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var out struct {
		AvatarURL string `json:"avatarUrl"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.AvatarURL, "X-Amz-Signature=") {
		t.Fatal("avatar viewing URL missing")
	}
	if user.AvatarURL != nil || *user.AvatarPath != key {
		t.Fatal("authenticated record was mutated")
	}
	var fields map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["AvatarPath"]; exists {
		t.Fatal("internal key leaked")
	}
	if _, exists := fields["avatarPath"]; exists {
		t.Fatal("internal key leaked")
	}
}
