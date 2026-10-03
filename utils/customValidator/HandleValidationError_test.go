package customValidator

import (
	"strings"
	"testing"
	"time"

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

type scheduleRequest struct {
	ArriveTime   *time.Time `json:"arriveTime" binding:"omitempty,arriveTime"`
	StayOverTime int        `json:"stayOverTime" binding:"stayMinutes"`
}

func newScheduleValidator() *validator.Validate {
	validate := validator.New()
	validate.SetTagName("binding")
	registerTagNames(validate)
	registerRules(validate)
	return validate
}

func TestScheduleTagsRejectBadValues(t *testing.T) {
	zero := time.Time{}
	err := newScheduleValidator().Struct(scheduleRequest{ArriveTime: &zero, StayOverTime: -5})
	if err == nil {
		t.Fatal("expected validation error")
	}
	got := HandleValidationError(err)
	if !strings.Contains(got.Errors["stayOverTime"], "minutes") {
		t.Fatalf("stay message = %q", got.Errors["stayOverTime"])
	}
	if !strings.Contains(got.Errors["arriveTime"], "timestamp") {
		t.Fatalf("arrive message = %q", got.Errors["arriveTime"])
	}
}

func TestScheduleTagsAcceptZeroStayAndOmittedArrive(t *testing.T) {
	validate := newScheduleValidator()
	if err := validate.Struct(scheduleRequest{StayOverTime: 0}); err != nil {
		t.Fatalf("omitted arrive: %v", err)
	}
	arrive := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	if err := validate.Struct(scheduleRequest{ArriveTime: &arrive, StayOverTime: 45}); err != nil {
		t.Fatalf("valid schedule: %v", err)
	}
}
