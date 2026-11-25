package objectStorage

import (
	"context"
	"errors"
	"testing"
)

const testImagePath = "user/550e8400-e29b-41d4-a716-446655440000.png"
const oldImagePath = "user/550e8400-e29b-41d4-a716-446655440001.jpg"

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
func (s *stubRecords) ImagePath(context.Context, string) (string, error) { return s.path, nil }
func (s *stubRecords) ClearImagePath(context.Context, string) error {
	s.path = ""
	s.cleared = true
	return nil
}

type stubStorage struct {
	namespace   string
	path        string
	opts        SignOptions
	info        ObjectInfo
	headErr     error
	headCalls   int
	deleted     []string
	deleteErr   error
	downloadErr error
}

func (s *stubStorage) SignUpload(_ context.Context, namespace, path string, opts SignOptions) (SignedUpload, error) {
	s.namespace, s.path, s.opts = namespace, path, opts
	return SignedUpload{URL: "https://storage.example/upload", Path: path}, nil
}
func (s *stubStorage) HeadObject(context.Context, string, string) (ObjectInfo, error) {
	s.headCalls++
	return s.info, s.headErr
}
func (s *stubStorage) Delete(_ context.Context, _ string, paths []string) error {
	s.deleted = append(s.deleted, paths...)
	return s.deleteErr
}
func (s *stubStorage) DownloadURL(context.Context, string, string) (string, error) {
	return "https://storage.example/view?signature=test", s.downloadErr
}

func TestRequestUploadUsesUniqueOwnedKeysAndPassesSizeAndType(t *testing.T) {
	storage := &stubStorage{}
	svc := NewImageService("avatar", storage, &stubRecords{})
	a, err := svc.RequestUpload(context.Background(), "user", "image/png", 123)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.RequestUpload(context.Background(), "user", "image/png", 123)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path == b.Path {
		t.Fatal("uploads reuse a key")
	}
	if err := ValidateOwnedPath("user", a.Path); err != nil {
		t.Fatal(err)
	}
	if storage.namespace != "avatar" || storage.opts.ContentType != "image/png" || storage.opts.SizeBytes != 123 {
		t.Fatalf("storage=%+v", storage)
	}
}

func TestImageUploadRejectsVideoAndAudio(t *testing.T) {
	svc := NewImageService("avatar", &stubStorage{}, &stubRecords{})
	for _, mime := range []string{"video/mp4", "audio/mpeg", "image/gif"} {
		_, err := svc.RequestUpload(context.Background(), "user", mime, 123)
		if !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("%s: %v", mime, err)
		}
	}
}

func TestConfirmRejectsForeignPathBeforeStorageAccess(t *testing.T) {
	storage := &stubStorage{}
	svc := NewImageService("avatar", storage, &stubRecords{})
	_, err := svc.Confirm(context.Background(), "other", testImagePath)
	if !errors.Is(err, ErrInvalidPath) || storage.headCalls != 0 || len(storage.deleted) != 0 {
		t.Fatalf("err=%v storage=%+v", err, storage)
	}
}

func TestConfirmStoresKeyThenDeletesPreviousImage(t *testing.T) {
	storage := &stubStorage{info: ObjectInfo{ContentType: "image/png", SizeBytes: 123}}
	records := &stubRecords{path: oldImagePath}
	svc := NewImageService("avatar", storage, records)
	viewURL, err := svc.Confirm(context.Background(), "user", testImagePath)
	if err != nil {
		t.Fatal(err)
	}
	if records.path != testImagePath || viewURL != "https://storage.example/view?signature=test" {
		t.Fatalf("stored=%s url=%s", records.path, viewURL)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != oldImagePath {
		t.Fatalf("deleted=%v", storage.deleted)
	}
}

func TestConfirmRequiresUploadedObject(t *testing.T) {
	svc := NewImageService("avatar", &stubStorage{headErr: ErrNotUploaded}, &stubRecords{})
	_, err := svc.Confirm(context.Background(), "user", testImagePath)
	if !errors.Is(err, ErrNotUploaded) {
		t.Fatal(err)
	}
}

func TestConfirmRejectsAndDeletesOversizedImageWithoutChangingRecord(t *testing.T) {
	storage := &stubStorage{info: ObjectInfo{ContentType: "image/png", SizeBytes: 5_000_001}}
	records := &stubRecords{path: oldImagePath}
	svc := NewImageService("avatar", storage, records)
	_, err := svc.Confirm(context.Background(), "user", testImagePath)
	if !errors.Is(err, ErrFileTooLarge) || records.path != oldImagePath {
		t.Fatalf("err=%v stored=%s", err, records.path)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != testImagePath {
		t.Fatalf("deleted=%v", storage.deleted)
	}
}

func TestConfirmRejectsMismatchedMetadata(t *testing.T) {
	for _, mime := range []string{"image/jpeg", "video/mp4", "text/html"} {
		storage := &stubStorage{info: ObjectInfo{ContentType: mime, SizeBytes: 123}}
		records := &stubRecords{path: oldImagePath}
		_, err := NewImageService("avatar", storage, records).Confirm(context.Background(), "user", testImagePath)
		if !errors.Is(err, ErrUnsupportedType) || records.path != oldImagePath || len(storage.deleted) != 1 {
			t.Fatalf("%s: err=%v storage=%+v", mime, err, storage)
		}
	}
}

func TestFailedSigningOrPersistenceKeepsPreviousImage(t *testing.T) {
	for _, failSigning := range []bool{true, false} {
		storage := &stubStorage{info: ObjectInfo{ContentType: "image/png", SizeBytes: 123}}
		records := &stubRecords{path: oldImagePath}
		if failSigning {
			storage.downloadErr = ErrSignFailed
		} else {
			records.setErr = errors.New("database unavailable")
		}
		_, err := NewImageService("avatar", storage, records).Confirm(context.Background(), "user", testImagePath)
		if err == nil || records.path != oldImagePath || len(storage.deleted) != 0 {
			t.Fatalf("err=%v records=%+v deleted=%v", err, records, storage.deleted)
		}
	}
}

func TestDeleteClearsRecordOnlyAfterStorageSucceeds(t *testing.T) {
	storage := &stubStorage{deleteErr: ErrDeleteFailed}
	records := &stubRecords{path: testImagePath}
	svc := NewImageService("avatar", storage, records)
	if err := svc.Delete(context.Background(), "user"); !errors.Is(err, ErrDeleteFailed) {
		t.Fatal(err)
	}
	if records.cleared {
		t.Fatal("record cleared after failed storage delete")
	}
	storage.deleteErr = nil
	if err := svc.Delete(context.Background(), "user"); err != nil {
		t.Fatal(err)
	}
	if !records.cleared || records.path != "" {
		t.Fatal("record not cleared")
	}
}
