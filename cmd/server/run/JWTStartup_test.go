package run

import (
	"errors"
	"testing"

	"Road-To-Destination-BE/module/authentication/service"
)

func TestRunRejectsWeakJWTSecretBeforeConnecting(t *testing.T) {
	t.Setenv("SECRET_JWT", "weak")
	if err := Run(); !errors.Is(err, service.ErrJWTConfiguration) {
		t.Fatalf("server returned %v, want JWT configuration error", err)
	}
}
