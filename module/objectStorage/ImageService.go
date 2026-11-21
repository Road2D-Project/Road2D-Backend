package objectStorage

import (
	"context"
	"log"
)

type ImageRepo interface {
	SetImagePath(ctx context.Context, objectID, newPath string) (oldPath string, err error)
	ImagePath(ctx context.Context, objectID string) (string, error)
	ClearImagePath(ctx context.Context, objectID string) error
}

type ImageService struct {
	media   *MediaService
	storage MediaClient
	records ImageRepo
}

func NewImageService(namespace string, storage MediaClient, records ImageRepo) *ImageService {
	return &ImageService{media: NewMediaService(namespace, storage, ImageExt), storage: storage, records: records}
}

func (s *ImageService) RequestUpload(ctx context.Context, objectID, contentType string, sizeBytes int64) (SignedUpload, error) {
	return s.media.RequestUpload(ctx, objectID, contentType, sizeBytes)
}

func (s *ImageService) Confirm(ctx context.Context, objectID, path string) (string, error) {
	viewURL, err := s.media.Confirm(ctx, objectID, path)
	if err != nil {
		return "", err
	}
	old, err := s.records.SetImagePath(ctx, objectID, path)
	if err != nil {
		return "", err
	}
	if old != "" && old != path {
		if err := s.storage.Delete(ctx, s.media.namespace, []string{old}); err != nil {
			log.Printf("delete replaced media object: %v", err)
		}
	}
	return viewURL, nil
}

func (s *ImageService) Delete(ctx context.Context, objectID string) error {
	path, err := s.records.ImagePath(ctx, objectID)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	if err := ValidateOwnedPath(objectID, path); err != nil {
		return err
	}
	if err := s.storage.Delete(ctx, s.media.namespace, []string{path}); err != nil {
		return err
	}
	return s.records.ClearImagePath(ctx, objectID)
}
