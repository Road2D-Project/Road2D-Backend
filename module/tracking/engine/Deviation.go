package engine

import "time"

// Deviation and redirect tuning. The room state machine and the snap loop
// read these; geo.OffRouteM and geo.ArriveM hold the distance thresholds.
const (
	// ConfirmSamples is how many consecutive off-route samples enter DEVIATED.
	ConfirmSamples = 3
	// ClearSamples is how many consecutive on-route samples return to OnROUTE.
	ClearSamples = 2
	// SampleMinInterval drops a GPS sample that arrives sooner after the previous one.
	SampleMinInterval = 900 * time.Millisecond
	// LivesStart is the life count of a fresh rider.
	LivesStart = 3
	// ImmuneDuration is how long a rider keeps lives after losing one.
	ImmuneDuration = 30 * time.Second
	// RedirectRefreshMoveM is how far from the redirect origin the rider must
	// move before the route is computed again.
	RedirectRefreshMoveM = 40.0
	// RedirectRefreshCooldown is the minimum gap between two router calls while
	// the rider stays deviated.
	RedirectRefreshCooldown = 20 * time.Second
	// RedirectRetryAfter schedules one retry after a provider error.
	RedirectRetryAfter = 10 * time.Second
	// RedirectTimeout bounds one router call.
	RedirectTimeout = 10 * time.Second

	redirectResultBuffer = 2
)
