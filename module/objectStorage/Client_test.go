package objectStorage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSignUploadMissingConfig(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SECRET_KEY", "")

	_, err := NewClient().SignUpload(context.Background(), "image", "avatar/a/a.png", SignOptions{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v", err)
	}
}

func TestSignUploadRejectsParentSegment(t *testing.T) {
	t.Setenv("SUPABASE_URL", "http://storage.example")
	t.Setenv("SUPABASE_SECRET_KEY", "secret")

	_, err := NewClient().SignUpload(context.Background(), "image", "avatar/../secret.png", SignOptions{})
	if !errors.Is(err, ErrInvalidObjectPath) {
		t.Fatalf("got %v", err)
	}
}

func TestSignUploadReturnsAbsoluteURL(t *testing.T) {
	const secret = "service-role-secret"
	var gotKey, gotAuth, gotUpsert, gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("apikey")
		gotAuth = r.Header.Get("Authorization")
		gotUpsert = r.Header.Get("x-upsert")
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"url":"/object/upload/sign/image/avatar/user/user.png?token=abc"}`)
	}))
	defer server.Close()

	t.Setenv("SUPABASE_URL", server.URL)
	t.Setenv("SUPABASE_SECRET_KEY", secret)

	signed, err := NewClient().SignUpload(context.Background(), "image", "/avatar/user/user.png", SignOptions{Upsert: true})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if gotKey != secret || gotAuth != "Bearer "+secret {
		t.Fatalf("auth headers key=%q auth=%q", gotKey, gotAuth)
	}
	if gotUpsert != "true" {
		t.Fatalf("upsert header = %q", gotUpsert)
	}
	if gotPath != "/storage/v1/object/upload/sign/image/avatar/user/user.png" {
		t.Fatalf("path = %s", gotPath)
	}
	wantURL := server.URL + "/storage/v1/object/upload/sign/image/avatar/user/user.png?token=abc"
	if signed.URL != wantURL {
		t.Fatalf("url = %s", signed.URL)
	}
	if signed.Path != "avatar/user/user.png" {
		t.Fatalf("stored path = %s", signed.Path)
	}
}

func TestSignUploadHidesUpstreamBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `secret-body`)
	}))
	defer server.Close()

	t.Setenv("SUPABASE_URL", server.URL)
	t.Setenv("SUPABASE_SECRET_KEY", "secret")

	_, err := NewClient().SignUpload(context.Background(), "image", "avatar/a/a.png", SignOptions{})
	if !errors.Is(err, ErrSignFailed) {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "secret-body") {
		t.Fatalf("upstream body leaked: %v", err)
	}
}

func TestPublicObjectURL(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://project.supabase.co/")
	t.Setenv("SUPABASE_SECRET_KEY", "secret")

	got, err := NewClient().PublicObjectURL("image", "/avatar/user/user.png")
	if err != nil {
		t.Fatalf("public url: %v", err)
	}
	want := "https://project.supabase.co/storage/v1/object/public/image/avatar/user/user.png"
	if got != want {
		t.Fatalf("public url = %s", got)
	}
}

func TestDeleteSendsPrefixes(t *testing.T) {
	const secret = "service-role-secret"
	var gotMethod, gotKey, gotPath, gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotKey = r.Header.Get("apikey")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()

	t.Setenv("SUPABASE_URL", server.URL)
	t.Setenv("SUPABASE_SECRET_KEY", secret)

	err := NewClient().Delete(context.Background(), "image", []string{"/avatar/user/user.png"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gotMethod != http.MethodDelete || gotKey != secret {
		t.Fatalf("method=%s key=%s", gotMethod, gotKey)
	}
	if gotPath != "/storage/v1/object/image" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody != `{"prefixes":["avatar/user/user.png"]}` {
		t.Fatalf("body = %s", gotBody)
	}
}

func TestDeleteHidesUpstreamBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `secret-body`)
	}))
	defer server.Close()

	t.Setenv("SUPABASE_URL", server.URL)
	t.Setenv("SUPABASE_SECRET_KEY", "secret")

	err := NewClient().Delete(context.Background(), "image", []string{"avatar/a/a.png"})
	if !errors.Is(err, ErrDeleteFailed) {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "secret-body") {
		t.Fatalf("upstream body leaked: %v", err)
	}
}

func TestExistsUsesObjectInfo(t *testing.T) {
	const secret = "service-role-secret"
	var gotPath, gotKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("apikey")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"name":"avatar/user/user.png"}`)
	}))
	defer server.Close()

	t.Setenv("SUPABASE_URL", server.URL)
	t.Setenv("SUPABASE_SECRET_KEY", secret)

	ok, err := NewClient().Exists(context.Background(), "image", "avatar/user/user.png")
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !ok {
		t.Fatal("expected the object to exist")
	}
	if gotKey != secret {
		t.Fatalf("apikey = %q", gotKey)
	}
	if gotPath != "/storage/v1/object/info/image/avatar/user/user.png" {
		t.Fatalf("path = %s", gotPath)
	}
}

func TestImageExt(t *testing.T) {
	ext, ok := ImageExt("image/jpeg")
	if !ok || ext != ".jpg" {
		t.Fatalf("jpeg ext = %q ok=%v", ext, ok)
	}
	if _, ok := ImageExt("image/gif"); ok {
		t.Fatal("gif should be rejected")
	}
}
