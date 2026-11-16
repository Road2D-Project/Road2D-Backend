package main

import (
	"math"
	"sort"
)

const timeEps = 1e-6

// effect is the set of events active at one instant. Later rows in the file win.
type effect struct {
	byName map[string]Event
	order  []string
}

func (e effect) has(name string) bool {
	_, ok := e.byName[name]
	return ok
}

func effectsAt(events []Event, elapsed float64) effect {
	fx := effect{byName: map[string]Event{}}
	seen := map[string]bool{}
	for _, event := range events {
		if elapsed < event.At || elapsed >= event.At+event.Duration {
			continue
		}
		if !seen[event.Name] {
			seen[event.Name] = true
			fx.order = append(fx.order, event.Name)
		}
		fx.byName[event.Name] = event
	}
	return fx
}

func freezes(name string) bool {
	return name == evDisconnect || name == evPause || name == evStayMode
}

// frozenSeconds is the wall time inside [0, until] where the rider does not advance.
// Overlapping freeze events are counted once.
func frozenSeconds(events []Event, until float64) float64 {
	if until <= 0 {
		return 0
	}
	spans := make([][2]float64, 0)
	for _, event := range events {
		if !freezes(event.Name) {
			continue
		}
		start, end := event.At, event.At+event.Duration
		if start < 0 {
			start = 0
		}
		if end > until {
			end = until
		}
		if end <= start {
			continue
		}
		spans = append(spans, [2]float64{start, end})
	}
	if len(spans) == 0 {
		return 0
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	curStart, curEnd := spans[0][0], spans[0][1]
	total := 0.0
	for _, span := range spans[1:] {
		if span[0] <= curEnd {
			if span[1] > curEnd {
				curEnd = span[1]
			}
			continue
		}
		total += curEnd - curStart
		curStart, curEnd = span[0], span[1]
	}
	return total + curEnd - curStart
}

// alongMeters is how far the rider has walked. Freeze windows do not add distance.
func alongMeters(motion Motion, events []Event, elapsed, length float64) float64 {
	moved := elapsed - frozenSeconds(events, elapsed)
	if moved < 0 {
		moved = 0
	}
	dist := moved * motion.SpeedMps
	if length > 0 && motion.Loop {
		return math.Mod(dist, length)
	}
	if dist > length {
		return length
	}
	return dist
}

// Sample is the position the phone would report at elapsed wall seconds.
type Sample struct {
	Point   Point
	Bearing float64
	AlongM  float64
	Battery int
	Signal  int
	Effect  effect
}

func sampleAt(route Route, motion Motion, events []Event, elapsed float64) Sample {
	along := alongMeters(motion, events, elapsed, route.Length)
	point, dir := route.at(along)
	fx := effectsAt(events, elapsed)
	if event, ok := fx.byName[evDeviate]; ok {
		point = offsetMeters(point, dir, deviateMeters(event))
	}
	battery := clampBattery(motion.BatteryPercent)
	signal := motion.SignalDbm
	if event, ok := fx.byName[evLowBattery]; ok {
		battery = batteryOf(event)
	}
	if event, ok := fx.byName[evWeakSignal]; ok {
		signal = signalOf(event)
	}
	return Sample{Point: point, Bearing: dir, AlongM: along, Battery: battery, Signal: signal, Effect: fx}
}

// horizon is the dry-run end: the path has been walked and every event has ended.
// Freeze time is added back, because those seconds do not consume the polyline.
func horizon(route Route, motion Motion, events []Event) float64 {
	last := 0.0
	for _, event := range events {
		if end := event.At + event.Duration; end > last {
			last = end
		}
	}
	if motion.Loop {
		if last == 0 {
			return 60
		}
		return last
	}
	moved := (last - frozenSeconds(events, last)) * motion.SpeedMps
	if moved+0.01 < route.Length {
		last += (route.Length - moved) / motion.SpeedMps
	}
	return last
}

// nextMark is the next position tick or event edge after elapsed.
func nextMark(elapsed, interval float64, events []Event) float64 {
	next := math.Inf(1)
	if interval > 0 {
		step := math.Floor((elapsed+timeEps)/interval) + 1
		next = step * interval
	}
	for _, event := range events {
		for _, mark := range []float64{event.At, event.At + event.Duration} {
			if mark > elapsed+timeEps && mark < next {
				next = mark
			}
		}
	}
	if math.IsInf(next, 1) {
		return elapsed + interval
	}
	return next
}

func turnedOn(prev, cur effect) []Event {
	var out []Event
	for _, name := range cur.order {
		if _, ok := prev.byName[name]; ok {
			continue
		}
		out = append(out, cur.byName[name])
	}
	return out
}

func turnedOff(prev, cur effect) []string {
	var out []string
	for _, name := range prev.order {
		if _, ok := cur.byName[name]; ok {
			continue
		}
		out = append(out, name)
	}
	return out
}
