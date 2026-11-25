package objectStorage

import "errors"

var (
	ErrNotConfigured        = errors.New("R2 storage is not configured")
	ErrInvalidConfiguration = errors.New("invalid media configuration")
	ErrInspectFailed        = errors.New("storage metadata request failed")
	ErrInvalidSize          = errors.New("sizeBytes must be a positive integer")
	ErrFileTooLarge         = errors.New("file exceeds size limit")
	ErrInvalidObjectPath    = errors.New("invalid storage object path")
	ErrSignFailed           = errors.New("storage sign request failed")
	ErrDeleteFailed         = errors.New("storage delete request failed")
	ErrUnsupportedType      = errors.New("unsupported content type")
	ErrInvalidPath          = errors.New("invalid media path")
	ErrNotUploaded          = errors.New("file not uploaded")
)
