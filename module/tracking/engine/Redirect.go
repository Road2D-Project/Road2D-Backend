package engine

// RedirectResult is what the redirect goroutine hands back to the room.
// Generation matches RedirectState.Generation of the attempt that asked.
type RedirectResult struct {
	Generation uint64
	Polyline   string
	DistanceM  float64
	DurationS  float64
	Err        error
}

// OfferRedirectResult puts a result into the rider's buffer without blocking.
// When the buffer is full the oldest result is dropped: the room only acts on
// the generation it is waiting for, and a newer attempt supersedes an older one.
// Safe to call from any goroutine.
func (p *TravelProgress) OfferRedirectResult(result RedirectResult) {
	select {
	case p.redirects <- result:
		return
	default:
	}
	select {
	case <-p.redirects:
	default:
	}
	select {
	case p.redirects <- result:
	default:
	}
}

// DrainRedirectResults returns every result that has arrived. Call it from the
// room goroutine on every tick so a late answer never sits in the buffer.
func (p *TravelProgress) DrainRedirectResults() []RedirectResult {
	var out []RedirectResult
	for {
		select {
		case result := <-p.redirects:
			out = append(out, result)
		default:
			return out
		}
	}
}

// NextRedirectGeneration bumps the counter. Every new request, refresh, and
// clear takes a new generation so stale answers can be told apart.
func (p *TravelProgress) NextRedirectGeneration() uint64 {
	p.RedirectGeneration++
	return p.RedirectGeneration
}

// ClearRedirect forgets the temporary route and its snap cursor. The next
// generation is bumped so an answer still in flight is ignored.
func (p *TravelProgress) ClearRedirect() {
	p.Redirect = nil
	p.RedirectSeg = 0
	p.RedirectProgress = 0
	p.RedirectDistM = 0
	p.RedirectArrived = false
	p.NextRedirectGeneration()
}
