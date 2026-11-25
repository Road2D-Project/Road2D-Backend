package objectStorage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func configuredClient(t *testing.T) *Client {
	t.Helper()
	t.Setenv("R2_ACCOUNT_ID", "test-account")
	t.Setenv("R2_BUCKET_NAME", "media")
	t.Setenv("R2_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("R2_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("R2_ENDPOINT", "")
	for _, name := range []string{"MEDIA_MAX_IMAGE_BYTES", "MEDIA_MAX_VIDEO_BYTES", "MEDIA_MAX_AUDIO_BYTES"} {
		t.Setenv(name, "")
	}
	return NewClient()
}

func TestSignUploadMissingConfig(t *testing.T) {
	configuredClient(t)
	t.Setenv("R2_BUCKET_NAME", "")
	_, err := NewClient().SignUpload(context.Background(), "avatar", "user/file.png", SignOptions{ContentType: "image/png", SizeBytes: 123})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v", err)
	}
}

func TestSignUploadUsesR2AndSignsTypeAndExactSize(t *testing.T) {
	c := configuredClient(t)
	signed, err := c.SignUpload(context.Background(), "avatar", "user/file.png", SignOptions{ContentType: "image/png", SizeBytes: 123})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "test-account.r2.cloudflarestorage.com" || u.Path != "/media/avatar/user/file.png" {
		t.Fatalf("unexpected endpoint %s", u.Redacted())
	}
	if signed.Method != "PUT" || signed.Path != "user/file.png" || signed.ExpiresAt.IsZero() {
		t.Fatalf("missing upload metadata: %+v", signed)
	}
	if u.Query().Get("X-Amz-Expires") != "900" || u.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("missing expiring signature")
	}
	for _, header := range []string{"content-type", "content-length"} {
		if !strings.Contains(u.Query().Get("X-Amz-SignedHeaders"), header) {
			t.Fatalf("%s not signed: %s", header, u.Query().Get("X-Amz-SignedHeaders"))
		}
	}
	if signed.Headers["Content-Type"] != "image/png" || signed.Headers["Content-Length"] != "123" {
		t.Fatalf("headers = %v", signed.Headers)
	}
	if signed.Headers["Cache-Control"] != "private, no-store" {
		t.Fatalf("missing object cache policy: %v", signed.Headers)
	}
	if strings.Contains(signed.URL, "test-secret-key") {
		t.Fatal("secret exposed")
	}
}

func TestLegacyCredentialNamesWork(t *testing.T) {
	configuredClient(t)
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("R2_ACCESS_TOKEN", "legacy-access")
	t.Setenv("R2_SECRET_ACCESS_TOKEN", "legacy-secret")
	url, err := NewClient().DownloadURL(context.Background(), "trip", "owner/file.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "legacy-access") || strings.Contains(url, "legacy-secret") {
		t.Fatal("incorrect credential handling")
	}
}

func TestDownloadURLUsesSeparateNamespaceAndOneHourExpiry(t *testing.T) {
	c := configuredClient(t)
	avatar, err := c.DownloadURL(context.Background(), "avatar", "same/file.png")
	if err != nil {
		t.Fatal(err)
	}
	trip, err := c.DownloadURL(context.Background(), "trip", "same/file.png")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := url.Parse(avatar)
	b, _ := url.Parse(trip)
	if a.Path == b.Path || a.Query().Get("X-Amz-Expires") != "3600" {
		t.Fatal("namespace or expiry incorrect")
	}
}

func TestSignUploadRejectsInvalidInput(t *testing.T) {
	c := configuredClient(t)
	for _, tc := range []struct {
		namespace, path, mime string
		size                  int64
		want                  error
	}{
		{"avatar", "../file.png", "image/png", 123, ErrInvalidObjectPath},
		{"../avatar", "file.png", "image/png", 123, ErrInvalidObjectPath},
		{"avatar", "file.mp4", "image/png", 123, ErrUnsupportedType},
		{"avatar", "file.png", "image/png", 0, ErrInvalidSize},
		{"attachments", "file.mp4", "video/mp4", 25_000_001, ErrFileTooLarge},
		{"attachments", "file.mp3", "audio/mpeg", 5_000_001, ErrFileTooLarge},
	} {
		_, err := c.SignUpload(context.Background(), tc.namespace, tc.path, SignOptions{ContentType: tc.mime, SizeBytes: tc.size})
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.path, err, tc.want)
		}
	}
}

func localStorageClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	c := configuredClient(t)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c.s3 = s3.New(s3.Options{Region: "auto", BaseEndpoint: aws.String(server.URL), UsePathStyle: true, Credentials: c.s3.Options().Credentials, RetryMaxAttempts: 1})
	return c
}

func TestHeadObjectReadsMetadataWithoutDownloadingFile(t *testing.T) {
	c := localStorageClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" || r.URL.Path != "/media/avatar/owner/file.png" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("request not signed")
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "123")
	})
	info, err := c.HeadObject(context.Background(), "avatar", "owner/file.png")
	if err != nil || info.ContentType != "image/png" || info.SizeBytes != 123 {
		t.Fatalf("info=%+v err=%v", info, err)
	}
}

func TestHeadObjectDistinguishesMissingFromForbidden(t *testing.T) {
	for _, status := range []int{404, 403, 400} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			c := localStorageClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
			_, err := c.HeadObject(context.Background(), "avatar", "owner/file.png")
			want := ErrInspectFailed
			if status == 404 {
				want = ErrNotUploaded
			}
			if !errors.Is(err, want) {
				t.Fatalf("got %v want %v", err, want)
			}
		})
	}
}

func TestDeleteValidatesAllKeysBeforeDeleting(t *testing.T) {
	calls := 0
	c := localStorageClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(204) })
	err := c.Delete(context.Background(), "avatar", []string{"owner/file.png", "../outside.png"})
	if !errors.Is(err, ErrInvalidObjectPath) || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if err := c.Delete(context.Background(), "avatar", []string{"owner/file.png"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestDeleteHidesUpstreamErrorDetails(t *testing.T) {
	c := localStorageClient(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "private upstream credentials", 403) })
	err := c.Delete(context.Background(), "avatar", []string{"owner/file.png"})
	if !errors.Is(err, ErrDeleteFailed) || strings.Contains(err.Error(), "credentials") {
		t.Fatalf("got %v", err)
	}
}
