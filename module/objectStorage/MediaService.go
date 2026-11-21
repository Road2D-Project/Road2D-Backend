package objectStorage

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type MediaClient interface {
	SignUpload(context.Context, string, string, SignOptions) (SignedUpload, error)
	HeadObject(context.Context, string, string) (ObjectInfo, error)
	Delete(context.Context, string, []string) error
	DownloadURL(context.Context, string, string) (string, error)
}

var _ MediaClient = (*Client)(nil)

// MediaService validates uploads independently of the feature's database records.
// Use ImageExt for covers/avatars and MediaExt for future media attachments.
type MediaService struct {
	namespace string
	storage   MediaClient
	extOf     ConnectExt
	policy    MediaPolicy
	configErr error
}

func NewMediaService(namespace string, storage MediaClient, extOf ConnectExt) *MediaService {
	policy, err := LoadMediaPolicy()
	return &MediaService{namespace: namespace, storage: storage, extOf: extOf, policy: policy, configErr: err}
}

func (s *MediaService) RequestUpload(ctx context.Context, ownerID, contentType string, sizeBytes int64) (SignedUpload, error) {
	if s.configErr != nil {
		return SignedUpload{}, s.configErr
	}
	if !validOwner(ownerID) {
		return SignedUpload{}, ErrInvalidPath
	}
	if s.extOf == nil {
		return SignedUpload{}, ErrUnsupportedType
	}
	ext, ok := s.extOf(contentType)
	if !ok {
		return SignedUpload{}, ErrUnsupportedType
	}
	if err := s.policy.Validate(contentType, sizeBytes); err != nil {
		return SignedUpload{}, err
	}
	path := ownerID + "/" + uuid.NewString() + ext
	return s.storage.SignUpload(ctx, s.namespace, path, SignOptions{ContentType: contentType, SizeBytes: sizeBytes})
}

// Confirm checks ownership and actual object metadata before any feature saves the key.
// Content-Type is metadata; inspecting/transcoding media bytes is a separate concern.
func (s *MediaService) Confirm(ctx context.Context, ownerID, path string) (string, error) {
	if s.configErr != nil {
		return "", s.configErr
	}
	if err := ValidateOwnedPath(ownerID, path); err != nil {
		return "", err
	}
	info, err := s.storage.HeadObject(ctx, s.namespace, path)
	if err != nil {
		return "", err
	}
	validationErr := s.policy.Validate(info.ContentType, info.SizeBytes)
	if validationErr == nil {
		if s.extOf == nil {
			validationErr = ErrUnsupportedType
		} else {
			ext, ok := s.extOf(info.ContentType)
			if !ok || !strings.HasSuffix(path, ext) {
				validationErr = ErrUnsupportedType
			}
		}
	}
	if validationErr != nil {
		if err := s.storage.Delete(ctx, s.namespace, []string{path}); err != nil {
			return "", errors.Join(validationErr, err)
		}
		return "", validationErr
	}
	return s.storage.DownloadURL(ctx, s.namespace, path)
}

func validOwner(ownerID string) bool {
	if ownerID == "" || strings.ContainsAny(ownerID, "/\\") {
		return false
	}
	_, _, err := cleanObjectPath(ownerID)
	return err == nil
}

func ValidateOwnedPath(ownerID, path string) error {
	logical, _, err := cleanObjectPath(path)
	if err != nil || logical != path || !validOwner(ownerID) {
		return ErrInvalidPath
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] != ownerID {
		return ErrInvalidPath
	}
	dot := strings.LastIndex(parts[1], ".")
	if dot <= 0 || dot == len(parts[1])-1 {
		return ErrInvalidPath
	}
	fileID, err := uuid.Parse(parts[1][:dot])
	if err != nil || fileID.String() != parts[1][:dot] {
		return ErrInvalidPath
	}
	return nil
}
