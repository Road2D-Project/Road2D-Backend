package objectStorage

import (
	"context"
	"log"
	"strings"
)

// ImageClient is the storage surface ImageService uses. *Client implements it.
type ImageClient interface {
	SignUpload(ctx context.Context, bucket, objectPath string, opts SignOptions) (SignedUpload, error)
	Exists(ctx context.Context, bucket, objectPath string) (bool, error)
	Delete(ctx context.Context, bucket string, objectPaths []string) error
	PublicObjectURL(bucket, objectPath string) (string, error)
}

var _ ImageClient = (*Client)(nil)

// PathProvider names the bucket and the object key for one kind of image.
// ObjectPath returns a key inside the bucket, with no leading slash, and it ends with ext.
type PathProvider interface {
	Bucket() string
	ObjectPath(objectID, ext string) string
}

// ImageRepo stores the object key. The feature that owns the row implements this.
// SetImagePath saves newPath and returns the previous key.
// ImagePath returns the stored key, or "" when the row has none.
type ImageRepo interface {
	SetImagePath(ctx context.Context, objectID, newPath string) (oldPath string, err error)
	ImagePath(ctx context.Context, objectID string) (string, error)
	ClearImagePath(ctx context.Context, objectID string) error
}

// ImageService signs an upload, records the key after the file is in Storage, and deletes the old key.
// A feature wires one service per image kind: avatar, trip cover, background, message attachment.
type ImageService struct {
	paths   PathProvider
	storage ImageClient
	records ImageRepo
	extOf   ConnectExt
}

func NewImageService(paths PathProvider, storage ImageClient, records ImageRepo, ext ConnectExt) *ImageService {
	return &ImageService{paths: paths, storage: storage, records: records, extOf: ext}
}

// RequestUpload returns a URL the client can PUT to.
// Upsert is set so a stable key, such as avatar/<id>/<id>.png, can be replaced.
func (s *ImageService) RequestUpload(ctx context.Context, objectID, contentType string) (SignedUpload, error) {
	if s.extOf == nil {
		return SignedUpload{}, ErrUnsupportedType
	}
	ext, ok := s.extOf(contentType)
	if !ok {
		return SignedUpload{}, ErrUnsupportedType
	}
	path := s.paths.ObjectPath(objectID, ext)
	return s.storage.SignUpload(ctx, s.paths.Bucket(), path, SignOptions{Upsert: true})
}

// Confirm stores path after Storage has the object, then returns its public URL.
// path is the object key from SignedUpload.Path, not a URL and not the bucket name.
func (s *ImageService) Confirm(ctx context.Context, objectID, path string) (string, error) {
	logical, err := s.ownedPath(objectID, path)
	if err != nil {
		return "", err
	}
	ok, err := s.storage.Exists(ctx, s.paths.Bucket(), logical)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotUploaded
	}

	old, err := s.records.SetImagePath(ctx, objectID, logical)
	if err != nil {
		return "", err
	}
	if old != "" && old != logical {
		if err := s.storage.Delete(ctx, s.paths.Bucket(), []string{old}); err != nil {
			log.Printf("delete replaced object %s: %v", old, err)
		}
	}
	return s.storage.PublicObjectURL(s.paths.Bucket(), logical)
}

// Delete removes the stored object, then clears the key.
// Storage goes first so a failed delete still leaves the key for a retry.
func (s *ImageService) Delete(ctx context.Context, objectID string) error {
	path, err := s.records.ImagePath(ctx, objectID)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	if err := s.storage.Delete(ctx, s.paths.Bucket(), []string{path}); err != nil {
		return err
	}
	return s.records.ClearImagePath(ctx, objectID)
}

func (s *ImageService) ownedPath(objectID, path string) (string, error) {
	logical, _, err := cleanObjectPath(path)
	if err != nil {
		return "", ErrInvalidPath
	}
	ext := fileExt(logical)
	if ext == "" || logical != s.paths.ObjectPath(objectID, ext) {
		return "", ErrInvalidPath
	}
	return logical, nil
}

func fileExt(path string) string {
	dot := strings.LastIndex(path, ".")
	slash := strings.LastIndex(path, "/")
	if dot <= slash || dot == len(path)-1 {
		return ""
	}
	return path[dot:]
}
