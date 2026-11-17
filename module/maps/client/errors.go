package client

import "errors"

var (
	ErrMissingAPIKey         = errors.New("goong calc API key is not set")
	ErrRateLimited           = errors.New("goong rate limited")
	ErrEmptyRoute            = errors.New("goong returned no routes")
	ErrEmptyTrip             = errors.New("goong returned no trips")
	ErrGoongStatus           = errors.New("goong returned a non-OK status")
	ErrProviderQuotaExceeded = errors.New("map provider quota exceeded")
	ErrQuotaUnavailable      = errors.New("map provider quota temporarily unavailable")
)
