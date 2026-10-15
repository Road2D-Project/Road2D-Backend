package objectStorage

// imageExts maps a content type to the object-key suffix a caller should use.
// SignUpload does not check this list: each caller decides which types it allows.
var imageExts = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// ImageExt returns the file suffix for a supported image content type.
func ImageExt(contentType string) (string, bool) {
	ext, ok := imageExts[contentType]
	return ext, ok
}

type ConnectExt func(contentType string) (string, bool)
