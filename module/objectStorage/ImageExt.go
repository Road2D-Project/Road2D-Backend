package objectStorage

import "mime"

// imageExts maps a content type to the object-key suffix a caller should use.
// Avatars and trip covers remain image-only.
var imageExts = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// ImageExt returns the file suffix for a supported image content type.
func ImageExt(contentType string) (string, bool) {
	contentType, err := canonicalContentType(contentType)
	if err != nil {
		return "", false
	}
	ext, ok := imageExts[contentType]
	return ext, ok
}

type ConnectExt func(contentType string) (string, bool)

// MediaExt is the allowlist for future video/audio attachment consumers.
func MediaExt(contentType string) (string, bool) {
	contentType, err := canonicalContentType(contentType)
	if err != nil {
		return "", false
	}
	if ext, ok := ImageExt(contentType); ok {
		return ext, true
	}
	switch contentType {
	case "video/mp4":
		return ".mp4", true
	case "video/webm":
		return ".webm", true
	case "audio/mpeg":
		return ".mp3", true
	case "audio/mp4":
		return ".m4a", true
	case "audio/aac":
		return ".aac", true
	case "audio/ogg":
		return ".ogg", true
	case "audio/webm":
		return ".webm", true
	case "audio/wav":
		return ".wav", true
	default:
		return "", false
	}
}

func canonicalContentType(contentType string) (string, error) {
	base, _, err := mime.ParseMediaType(contentType)
	return base, err
}
