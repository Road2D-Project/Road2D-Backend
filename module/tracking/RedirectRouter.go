package tracking

import "context"

// RedirectRouter computes one road route from a GPS point to a stop.
// It returns the encoded polyline (precision 1e5), the length in meters, and
// the ride time in seconds. The implementation must not write the result into
// the leg or travel caches: the origin is a raw GPS fix, not a pin.
type RedirectRouter interface {
	Redirect(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (polyline string, distanceM, durationS float64, err error)
}
