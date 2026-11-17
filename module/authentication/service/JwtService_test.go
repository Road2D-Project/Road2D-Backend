package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

const (
	testAccessSecret  = "access-test-secret-01234567890123456789"
	testRefreshSecret = "refresh-test-secret-01234567890123456789"
	testMailSecret    = "mail-test-secret-01234567890123456789"
)

func setTestJWTSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("SECRET_JWT", testAccessSecret)
	t.Setenv("SECRET_REFRESH_JWT", testRefreshSecret)
	t.Setenv("SECRET_MAIL_JWT", testMailSecret)
}

func testSessionUser() *model.User {
	user := &model.User{Username: "traveler", Email: "traveler@example.com", SessionVersion: 1}
	user.ID = uuid.MustParse("b321b53a-1bc8-4f0c-9395-637b71cb211d")
	return user
}

type stubSessionUsers struct {
	user *model.User
	err  error
}

func (s *stubSessionUsers) FindUserByUsername(context.Context, string) (*model.User, error) {
	return s.user, s.err
}

func TestValidateJWTSecretsRejectsMissingWeakAndSharedKeys(t *testing.T) {
	for _, test := range []struct{ name, value string }{
		{"missing", ""},
		{"short", "weak-secret"},
		{"shared", testRefreshSecret},
		{"whitespace", "                                "},
	} {
		t.Run(test.name, func(t *testing.T) {
			setTestJWTSecrets(t)
			t.Setenv("SECRET_JWT", test.value)
			if err := ValidateJWTSecrets(); !errors.Is(err, ErrJWTConfiguration) {
				t.Fatalf("got %v, want configuration error", err)
			}
		})
	}
	setTestJWTSecrets(t)
	if err := ValidateJWTSecrets(); err != nil {
		t.Fatal(err)
	}
}

func TestJWTRejectsOtherSigningAlgorithms(t *testing.T) {
	setTestJWTSecrets(t)
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512, jwt.SigningMethodNone} {
		t.Run(method.Alg(), func(t *testing.T) {
			claims, err := sessionClaims(testSessionUser(), "refresh", time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			var key interface{} = []byte(testRefreshSecret)
			if method == jwt.SigningMethodNone {
				key = jwt.UnsafeAllowNoneSignatureType
			}
			raw, err := jwt.NewWithClaims(method, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyRefreshToken(raw); !errors.Is(err, ErrTokenInvalid) {
				t.Fatalf("got %v, want invalid token", err)
			}
		})
	}
}

func TestJWTRequiresExpiryIdentityVersionAndPurpose(t *testing.T) {
	setTestJWTSecrets(t)
	for _, field := range []string{"exp", "sub", "username", "sessionVersion", "tokenUse"} {
		t.Run(field, func(t *testing.T) {
			claims, err := sessionClaims(testSessionUser(), "refresh", time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			delete(claims, field)
			raw, err := signClaims(claims, "SECRET_REFRESH_JWT")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyRefreshToken(raw); !errors.Is(err, ErrTokenInvalid) {
				t.Fatalf("missing %s returned %v", field, err)
			}
		})
	}
	for _, version := range []interface{}{0, -1, 1.5, "1"} {
		claims, err := sessionClaims(testSessionUser(), "refresh", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		claims["sessionVersion"] = version
		raw, err := signClaims(claims, "SECRET_REFRESH_JWT")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyRefreshToken(raw); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("version %v returned %v", version, err)
		}
	}
}

func TestRefreshTokensIdentifyIndividualLogins(t *testing.T) {
	setTestJWTSecrets(t)
	first, err := CreateRefreshToken(testSessionUser())
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateRefreshToken(testSessionUser())
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("independent login sessions received identical refresh tokens")
	}
}

func TestRefreshRejectsResetSessionsAndRecreatedAccounts(t *testing.T) {
	setTestJWTSecrets(t)
	users := &stubSessionUsers{user: testSessionUser()}
	raw, err := CreateRefreshToken(users.user)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewLogoutService(&stubRevokeStore{}, users)
	if _, err := svc.AssertRefreshUsable(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	users.user.SessionVersion++
	if _, err := svc.AssertRefreshUsable(context.Background(), raw); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("token from before password reset returned %v", err)
	}
	users.user.SessionVersion = 1
	users.user.ID = uuid.New()
	if _, err := svc.AssertRefreshUsable(context.Background(), raw); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("token for recreated username returned %v", err)
	}
}

func TestRefreshFailsClosedWhenSessionLookupIsUnavailable(t *testing.T) {
	setTestJWTSecrets(t)
	raw, err := CreateRefreshToken(testSessionUser())
	if err != nil {
		t.Fatal(err)
	}
	svc := NewLogoutService(&stubRevokeStore{}, &stubSessionUsers{err: errors.New("database unavailable")})
	if _, err := svc.AssertRefreshUsable(context.Background(), raw); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("got %v, want unavailable authentication", err)
	}
}
