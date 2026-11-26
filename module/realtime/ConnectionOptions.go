package realtime

import "time"

// ConnectionOptions is immutable authentication metadata. It does not authorize
// a join or establish a revocation subscription; those belong to the caller.
type ConnectionOptions struct {
	TokenID        string
	SessionVersion int64
	ExpiresAt      time.Time
}

func (o ConnectionOptions) Validate() error {
	if o.TokenID == "" || len(o.TokenID) > 128 || o.SessionVersion < 1 || o.ExpiresAt.IsZero() {
		return ErrInvalidConnectionOptions
	}
	if !o.ExpiresAt.After(time.Now()) {
		return ErrConnectionExpired
	}
	return nil
}
