package service

import (
	"testing"
	"time"
)

func TestTokenPairSharesLoginIdentityAndRefreshPreservesIt(t *testing.T) {
	setTestJWTSecrets(t)
	user := testSessionUser()
	access, refresh, err := CreateTokenPair(user)
	if err != nil {
		t.Fatal(err)
	}
	a, err := VerifyAccessToken(access)
	if err != nil {
		t.Fatal(err)
	}
	r, err := VerifyRefreshToken(refresh)
	if err != nil {
		t.Fatal(err)
	}
	if a["sid"] == "" || a["sid"] != r["sid"] || a["jti"] == r["jti"] {
		t.Fatal("credentials were not linked to one login with distinct token IDs")
	}
	next, err := CreateAccessFromRefresh(user, refresh)
	if err != nil {
		t.Fatal(err)
	}
	n, err := VerifyAccessToken(next)
	if err != nil || n["sid"] != r["sid"] {
		t.Fatal("refresh changed login identity")
	}
	r["exp"] = time.Now().Add(time.Minute).Unix()
	nearExpiry, err := signClaims(r, "SECRET_REFRESH_JWT")
	if err != nil {
		t.Fatal(err)
	}
	next, err = CreateAccessFromRefresh(user, nearExpiry)
	if err != nil {
		t.Fatal(err)
	}
	n, err = VerifyAccessToken(next)
	if err != nil {
		t.Fatal(err)
	}
	limit, _ := ClaimsExpiry(r)
	actual, _ := ClaimsExpiry(n)
	if actual.After(limit) {
		t.Fatal("access token outlived the login denylist lifetime")
	}
	delete(r, "sid")
	legacy, err := signClaims(r, "SECRET_REFRESH_JWT")
	if err != nil {
		t.Fatal(err)
	}
	next, err = CreateAccessFromRefresh(user, legacy)
	if err != nil {
		t.Fatal(err)
	}
	n, err = VerifyAccessToken(next)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := n["sid"]; exists {
		t.Fatal("legacy refresh was silently assigned an unrelated login ID")
	}
}
