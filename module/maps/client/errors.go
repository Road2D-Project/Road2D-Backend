package client

import "errors"

var (
	ErrMissingAPIKey = errors.New("goong calc API key is not set")
	ErrRateLimited    = errors.New("goong rate limited")
	ErrEmptyRoute     = errors.New("goong returned no routes")
	ErrGoongStatus    = errors.New("goong returned a non-OK status")
)
