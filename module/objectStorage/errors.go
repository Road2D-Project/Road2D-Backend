package objectStorage

import "errors"

var (
	ErrNotConfigured     = errors.New("supabase storage is not configured")
	ErrInvalidObjectPath = errors.New("invalid storage object path")
	ErrSignFailed        = errors.New("storage sign request failed")
	ErrDeleteFailed      = errors.New("storage delete request failed")
	ErrUnsupportedType   = errors.New("unsupported content type")
	ErrInvalidPath       = errors.New("invalid image path")
	ErrNotUploaded       = errors.New("file not uploaded")
)
