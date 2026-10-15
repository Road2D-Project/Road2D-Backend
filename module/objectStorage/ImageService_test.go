package objectStorage

import (
	"context"
	"errors"
	"testing"
)

type stubPaths struct{}

func (stubPaths) Bucket() string { return "image" }

func (stubPaths) ObjectPath(objectID, ext string) string {
	return "avatar/" + objectID + "/" + objectID + ext
}

type stubRecords struct {
	path    string
	setErr  error
	cleared bool
}

func (s *stubRecords) SetImagePath(_ context.Context, _, newPath string) (string, error) {
	if s.setErr != nil {
		return "", s.setErr
	}
	old := s.path
	s.path = newPath
	return old, nil
}

func (s *stubRecords) ImagePath(context.Context, string) (string, error) {
	return s.path, nil
}

func (s *stubRecords) ClearImagePath(context.Context, string) error {
	s.path = ""
	s.cleared = true
	return nil
}

type stubStorage struct {
	bucket  string
	path    string
	upsert  bool
	exists  bool
	deleted []string
}

func (s *stubStorage) SignUpload(_ context.Context, bucket, objectPath string, opts SignOptions) (SignedUpload, error) {
	s.bucket = bucket
	s.path = objectPath
	s.upsert = opts.Upsert
	return SignedUpload{URL: "https://storage.example/upload", Path: objectPath}, nil
}

func (s *stubStorage) Exists(context.Context, string, string) (bool, error) {
	return s.exists, nil
}

func (s *stubStorage) Delete(_ context.Context, _ string, objectPaths []string) error {
	s.deleted = append(s.deleted, objectPaths...)
	return nil
}

func (s *stubStorage) PublicObjectURL(bucket, objectPath string) (string, error) {
	return "https://storage.example/public/" + bucket + "/" + objectPath, nil
}

func TestRequestUploadSignsBucketAndStablePath(t *testing.T) {
	storage := &stubStorage{}
	svc := NewImageService(stubPaths{}, storage, &stubRecords{}, ImageExt)

	signed, err := svc.RequestUpload(context.Background(), "user", "image/png")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if storage.bucket != "image" || storage.path != "avatar/user/user.png" || !storage.upsert {
		t.Fatalf("sign bucket=%s path=%s upsert=%v", storage.bucket, storage.path, storage.upsert)
	}
	if signed.Path != "avatar/user/user.png" {
		t.Fatalf("path = %s", signed.Path)
	}
}

func TestRequestUploadRejectsUnknownType(t *testing.T) {
	svc := NewImageService(stubPaths{}, &stubStorage{}, &stubRecords{}, ImageExt)
	_, err := svc.RequestUpload(context.Background(), "user", "image/gif")
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("got %v", err)
	}
}

func TestConfirmRejectsForeignPath(t *testing.T) {
	svc := NewImageService(stubPaths{}, &stubStorage{exists: true}, &stubRecords{}, ImageExt)
	_, err := svc.Confirm(context.Background(), "user", "avatar/other/other.png")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("got %v", err)
	}
}

func TestConfirmStoresKeyAndDeletesPrevious(t *testing.T) {
	storage := &stubStorage{exists: true}
	records := &stubRecords{path: "avatar/user/user.jpg"}
	svc := NewImageService(stubPaths{}, storage, records, ImageExt)

	publicURL, err := svc.Confirm(context.Background(), "user", "/avatar/user/user.png")
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if records.path != "avatar/user/user.png" {
		t.Fatalf("stored = %s", records.path)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != "avatar/user/user.jpg" {
		t.Fatalf("deleted = %v", storage.deleted)
	}
	if publicURL != "https://storage.example/public/image/avatar/user/user.png" {
		t.Fatalf("url = %s", publicURL)
	}
}

func TestConfirmRequiresUploadedObject(t *testing.T) {
	svc := NewImageService(stubPaths{}, &stubStorage{}, &stubRecords{}, ImageExt)
	_, err := svc.Confirm(context.Background(), "user", "avatar/user/user.png")
	if !errors.Is(err, ErrNotUploaded) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteRemovesStoredKeyThenClearsIt(t *testing.T) {
	storage := &stubStorage{}
	records := &stubRecords{path: "avatar/user/user.png"}
	svc := NewImageService(stubPaths{}, storage, records, ImageExt)

	if err := svc.Delete(context.Background(), "user"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != "avatar/user/user.png" {
		t.Fatalf("deleted = %v", storage.deleted)
	}
	if !records.cleared || records.path != "" {
		t.Fatalf("cleared=%v path=%s", records.cleared, records.path)
	}
}
