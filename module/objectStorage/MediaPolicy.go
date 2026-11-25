package objectStorage

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type MediaPolicy struct {
	MaxImageBytes int64
	MaxVideoBytes int64
	MaxAudioBytes int64
}

func LoadMediaPolicy() (MediaPolicy, error) {
	policy := MediaPolicy{MaxImageBytes: 5_000_000, MaxVideoBytes: 25_000_000, MaxAudioBytes: 5_000_000}
	for name, limit := range map[string]*int64{
		"MEDIA_MAX_IMAGE_BYTES": &policy.MaxImageBytes,
		"MEDIA_MAX_VIDEO_BYTES": &policy.MaxVideoBytes,
		"MEDIA_MAX_AUDIO_BYTES": &policy.MaxAudioBytes,
	} {
		if raw := os.Getenv(name); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 {
				return MediaPolicy{}, fmt.Errorf("%w: %s must be a positive integer", ErrInvalidConfiguration, name)
			}
			*limit = value
		}
	}
	return policy, nil
}

func (p MediaPolicy) Validate(contentType string, sizeBytes int64) error {
	if _, ok := MediaExt(contentType); !ok {
		return ErrUnsupportedType
	}
	contentType, _ = canonicalContentType(contentType)
	if sizeBytes <= 0 {
		return ErrInvalidSize
	}
	var limit int64
	switch {
	case strings.HasPrefix(contentType, "image/"):
		limit = p.MaxImageBytes
	case strings.HasPrefix(contentType, "video/"):
		limit = p.MaxVideoBytes
	case strings.HasPrefix(contentType, "audio/"):
		limit = p.MaxAudioBytes
	}
	if sizeBytes > limit {
		return fmt.Errorf("%w: maximum %d bytes", ErrFileTooLarge, limit)
	}
	return nil
}
