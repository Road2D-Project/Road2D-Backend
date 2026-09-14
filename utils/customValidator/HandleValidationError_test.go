package customValidator

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

type sampleRequest struct {
	Username string `json:"username" validate:"required,min=4"`
	Email    string `json:"email" validate:"required,email"`
}

func TestHandleValidationErrorIncludesFieldNames(t *testing.T) {
	validate := validator.New()
	registerTagNames(validate)

	err := validate.Struct(sampleRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	got := HandleValidationError(err)
	if got.Status != 400 {
		t.Fatalf("status = %d", got.Status)
	}
	if got.Errors["username"] == "" || !strings.Contains(got.Errors["username"], "username") {
		t.Fatalf("username message = %q", got.Errors["username"])
	}
	if strings.Contains(got.Errors["username"], "This field") {
		t.Fatalf("should not use This field: %q", got.Errors["username"])
	}
	if got.Errors["email"] == "" || !strings.Contains(got.Errors["email"], "email") {
		t.Fatalf("email message = %q", got.Errors["email"])
	}
	if !strings.Contains(got.Message, "username") && !strings.Contains(got.Message, "email") {
		t.Fatalf("summary message missing field names: %q", got.Message)
	}
}
