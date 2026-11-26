package request

import "encoding/json"

// MutedUntil is required: an RFC3339 timestamp or explicit null to unmute.
type UpdatePreferencesRequest struct {
	MutedUntil json.RawMessage `json:"mutedUntil" binding:"required" swaggertype:"string" format:"date-time" extensions:"x-nullable"`
}
