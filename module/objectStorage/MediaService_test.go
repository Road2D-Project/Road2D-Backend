package objectStorage

import (
	"context"
	"errors"
	"testing"
)

func TestVideoAndAudioLimitsAtSigningAndConfirmation(t *testing.T) {
	for _, tc := range []struct {
		mime, ext string
		max       int64
	}{
		{"video/mp4", ".mp4", 25_000_000},
		{"video/webm", ".webm", 25_000_000},
		{"video/webm; codecs=vp9", ".webm", 25_000_000},
		{"audio/mpeg", ".mp3", 5_000_000},
		{"audio/webm; codecs=opus", ".webm", 5_000_000},
		{"audio/mp4", ".m4a", 5_000_000},
	} {
		t.Run(tc.mime, func(t *testing.T) {
			storage := &stubStorage{info: ObjectInfo{ContentType: tc.mime, SizeBytes: tc.max}}
			svc := NewMediaService("attachments", storage, MediaExt)
			signed, err := svc.RequestUpload(context.Background(), "owner", tc.mime, tc.max)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Confirm(context.Background(), "owner", signed.Path); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.RequestUpload(context.Background(), "owner", tc.mime, tc.max+1); !errors.Is(err, ErrFileTooLarge) {
				t.Fatal(err)
			}
			storage.info.SizeBytes++
			if _, err := svc.Confirm(context.Background(), "owner", signed.Path); !errors.Is(err, ErrFileTooLarge) {
				t.Fatal(err)
			}
			if len(storage.deleted) != 1 || storage.deleted[0] != signed.Path {
				t.Fatal("oversized upload not deleted")
			}
		})
	}
}

func TestMediaLimitsAreConfigurableAndInvalidConfigFailsClosed(t *testing.T) {
	t.Setenv("MEDIA_MAX_VIDEO_BYTES", "42")
	svc := NewMediaService("attachments", &stubStorage{}, MediaExt)
	if _, err := svc.RequestUpload(context.Background(), "owner", "video/mp4", 43); !errors.Is(err, ErrFileTooLarge) {
		t.Fatal(err)
	}
	for _, raw := range []string{"0", "-1", "invalid"} {
		t.Setenv("MEDIA_MAX_VIDEO_BYTES", raw)
		_, err := NewMediaService("attachments", &stubStorage{}, MediaExt).RequestUpload(context.Background(), "owner", "video/mp4", 1)
		if !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}

func TestOwnedPathRejectsTraversalURLsAndExtraSegments(t *testing.T) {
	for _, path := range []string{
		"../user/file.png", "/" + testImagePath, "https://example.com/" + testImagePath,
		"user/../" + testImagePath, "user/extra/550e8400-e29b-41d4-a716-446655440000.png", "user/file.png",
	} {
		if err := ValidateOwnedPath("user", path); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("accepted %s", path)
		}
	}
}
