package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type printer struct {
	mu    sync.Mutex
	w     io.Writer
	quiet bool
}

func newPrinter(w io.Writer, quiet bool) *printer {
	if w == nil {
		w = os.Stdout
	}
	return &printer{w: w, quiet: quiet}
}

func (p *printer) rawf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.w, format+"\n", args...)
}

func (p *printer) timed(elapsed float64, kind, msg string) {
	p.rawf("t=%6.1fs  %-5s  %s", elapsed, kind, msg)
}

type options struct {
	Dry   bool
	Quiet bool
}

type runner struct {
	sc             Scenario
	route          Route
	out            *printer
	dry            bool
	quiet          bool
	start          time.Time
	endpoint       string
	token          string
	link           *link
	socketUp       bool
	everConnected  bool
	openedOnce     bool
	openedThisTick bool
	stayOn         bool
	publicOn       bool
	lastSend       float64
	end            float64
	prev           effect
}

// Run prints the plan, then walks the script. Dry-run stops when the path and the events are done.
// Live mode keeps the socket open until Ctrl+C, holding the last point.
func Run(ctx context.Context, sc Scenario, route Route, opt options, w io.Writer) error {
	out := newPrinter(w, opt.Quiet)
	printPlan(out, sc, route, opt.Dry)
	r := &runner{
		sc:       sc,
		route:    route,
		out:      out,
		dry:      opt.Dry,
		quiet:    opt.Quiet,
		lastSend: -1,
		end:      horizon(route, sc.Motion, sc.Events),
		prev:     effect{byName: map[string]Event{}},
	}
	if opt.Dry {
		if endpoint, err := roomURL(sc.Server, sc.RoomID); err == nil && requireRoom(sc.RoomID) == nil {
			r.endpoint = endpoint
		} else {
			r.endpoint = "(chưa có roomId)"
		}
	} else {
		if err := requireRoom(sc.RoomID); err != nil {
			return err
		}
		endpoint, err := roomURL(sc.Server, sc.RoomID)
		if err != nil {
			return err
		}
		r.endpoint = endpoint
		token, err := r.ensureToken(ctx)
		if err != nil {
			return err
		}
		r.token = token
	}
	r.start = time.Now()
	return r.loop(ctx)
}

func (r *runner) ensureToken(ctx context.Context) (string, error) {
	if r.sc.Auth.AccessToken != "" {
		r.out.rawf("auth      dùng access token sẵn có")
		return r.sc.Auth.AccessToken, nil
	}
	if r.sc.Auth.Username == "" || r.sc.Auth.Password == "" {
		return "", fmt.Errorf("cần accessToken hoặc cả username và password")
	}
	r.out.rawf("auth      đang đăng nhập %s", r.sc.Auth.Username)
	return login(ctx, r.sc.Server, r.sc.Auth.Username, r.sc.Auth.Password)
}

func (r *runner) loop(ctx context.Context) error {
	interval := float64(r.sc.Motion.SendEveryMs) / 1000
	virtual := 0.0
	for {
		if ctx.Err() != nil {
			r.shutdown(r.elapsed(virtual), "dừng theo Ctrl+C")
			return nil
		}
		elapsed := r.elapsed(virtual)
		if err := r.tick(ctx, elapsed); err != nil {
			if ctx.Err() != nil {
				r.shutdown(time.Since(r.start).Seconds(), "dừng theo Ctrl+C")
				return nil
			}
			r.closeLink()
			return err
		}
		if r.dry && elapsed >= r.end-timeEps {
			r.shutdown(elapsed, "hết dry-run")
			return nil
		}
		next := nextMark(elapsed, interval, r.sc.Events)
		if r.dry {
			if next > r.end {
				next = r.end
			}
			if next <= elapsed+timeEps {
				r.shutdown(elapsed, "hết dry-run")
				return nil
			}
			virtual = next
			continue
		}
		wait := next - time.Since(r.start).Seconds()
		if err := sleepCtx(ctx, wait); err != nil {
			r.shutdown(time.Since(r.start).Seconds(), "dừng theo Ctrl+C")
			return nil
		}
	}
}
func (r *runner) elapsed(virtual float64) float64 {
	if r.dry {
		return virtual
	}
	return time.Since(r.start).Seconds()
}

func (r *runner) tick(ctx context.Context, elapsed float64) error {
	r.openedThisTick = false
	current := sampleAt(r.route, r.sc.Motion, r.sc.Events, elapsed)
	for _, event := range turnedOn(r.prev, current.Effect) {
		r.out.timed(elapsed, "event", fmt.Sprintf("%s bật (%.1fs) — %s · along %.0f m", event.Name, event.Duration, describe(event), current.AlongM))
	}
	for _, name := range turnedOff(r.prev, current.Effect) {
		r.out.timed(elapsed, "event", fmt.Sprintf("%s tắt · along %.0f m", name, current.AlongM))
	}

	if r.socketUp && !r.dry && (r.link == nil || r.link.dead.Load()) {
		r.socketUp = false
		r.link = nil
	}
	wantOpen := !current.Effect.has(evDisconnect)
	if !wantOpen && r.socketUp {
		r.closeLink()
	}
	if wantOpen && !r.socketUp {
		if err := r.open(ctx, elapsed); err != nil {
			if !r.everConnected {
				return err
			}
			r.out.timed(elapsed, "retry", err.Error())
		}
	}

	r.syncBool(elapsed, current.Effect.has(evStayMode), &r.stayOn, "tracking.stayMode")
	r.syncBool(elapsed, current.Effect.has(evUserStayPublic), &r.publicOn, "tracking.userStayPublic")

	interval := float64(r.sc.Motion.SendEveryMs) / 1000
	due := r.lastSend < 0 || elapsed-r.lastSend >= interval-timeEps
	atEnd := r.dry && elapsed >= r.end-timeEps
	if wantOpen && r.socketUp && (due || r.openedThisTick || atEnd) {
		frame := positionFrame(current.Point, current.Battery, current.Signal)
		r.out.timed(elapsed, "send", fmt.Sprintf("%s  along=%.0fm", frame, current.AlongM))
		if !r.dry {
			if err := r.link.send([]byte(frame)); err != nil {
				r.out.timed(elapsed, "retry", err.Error())
				r.closeLink()
			}
		}
		if r.socketUp || r.dry {
			r.lastSend = elapsed
		}
	}
	r.prev = current.Effect
	return nil
}

func (r *runner) syncBool(elapsed float64, want bool, armed *bool, msgType string) {
	if want == *armed || !r.socketUp {
		return
	}
	frame := boolFrame(msgType, want)
	r.out.timed(elapsed, "send", frame)
	if !r.dry {
		if err := r.link.send([]byte(frame)); err != nil {
			r.out.timed(elapsed, "retry", err.Error())
			r.closeLink()
			return
		}
	}
	*armed = want
}

func (r *runner) open(ctx context.Context, elapsed float64) error {
	label := r.endpoint
	if r.openedOnce {
		label = "nối lại " + r.endpoint
	}
	if r.dry {
		r.socketUp = true
		r.everConnected = true
		r.openedOnce = true
		r.openedThisTick = true
		r.out.timed(elapsed, "dial", label)
		return nil
	}
	conn, err := dialSocket(ctx, r.endpoint, r.token)
	if err != nil {
		r.socketUp = false
		return fmt.Errorf("%s: %w", r.endpoint, err)
	}
	if r.link != nil {
		r.link.close()
	}
	next := &link{conn: conn}
	r.link = next
	r.socketUp = true
	r.everConnected = true
	r.openedOnce = true
	r.openedThisTick = true
	go next.readPump(r.out, r.start, r.quiet)
	r.out.timed(elapsed, "dial", label)
	return nil
}

func (r *runner) closeLink() {
	if r.link != nil {
		r.link.close()
		r.link = nil
	}
	r.socketUp = false
}

func (r *runner) shutdown(elapsed float64, reason string) {
	r.finishBool(elapsed, &r.stayOn, "tracking.stayMode", "stayMode còn bật trên server vì socket đang đóng")
	r.finishBool(elapsed, &r.publicOn, "tracking.userStayPublic", "userStayPublic còn true trên server vì socket đang đóng")
	r.closeLink()
	r.out.timed(elapsed, "stop", reason)
}

func (r *runner) finishBool(elapsed float64, armed *bool, msgType, stuck string) {
	if !*armed {
		return
	}
	if r.dry || r.socketUp {
		frame := boolFrame(msgType, false)
		r.out.timed(elapsed, "send", frame)
		if !r.dry && r.link != nil {
			_ = r.link.send([]byte(frame))
		}
	} else {
		r.out.timed(elapsed, "event", stuck)
	}
	*armed = false
}

func printPlan(out *printer, sc Scenario, route Route, dry bool) {
	name := sc.Name
	if name == "" {
		name = "(không tên)"
	}
	out.rawf("kịch bản  %s", name)
	if sc.Description != "" {
		out.rawf("mô tả     %s", sc.Description)
	}
	out.rawf("server    %s", sc.Server)
	room := sc.RoomID
	if room == "" {
		room = "(chưa đặt)"
	}
	out.rawf("room      %s", room)
	out.rawf("auth      %s", authLabel(sc))
	out.rawf("đường     %s · %d điểm · %.0f m", route.Source, len(route.Points), route.Length)
	repeat := "một lượt, rồi đứng ở điểm cuối"
	if sc.Motion.Loop {
		repeat = "lặp lại polyline"
	}
	out.rawf("chạy      %.1f m/s · nhịp %d ms · %s", sc.Motion.SpeedMps, sc.Motion.SendEveryMs, repeat)
	out.rawf("pin/sóng  %d%% · %d dBm", sc.Motion.BatteryPercent, sc.Motion.SignalDbm)
	if len(sc.Events) == 0 {
		out.rawf("sự kiện   không có")
	} else {
		out.rawf("sự kiện   %d", len(sc.Events))
		for _, event := range sc.Events {
			out.rawf("  %6.1fs  %5.1fs  %-16s  %s", event.At, event.Duration, event.Name, describe(event))
			if event.Note != "" {
				out.rawf("%37s%s", "", event.Note)
			}
		}
	}
	if dry {
		out.rawf("chế độ    dry-run, không mở socket")
	}
	out.rawf("")
}

func sleepCtx(ctx context.Context, seconds float64) error {
	if seconds < 0 {
		seconds = 0
	}
	timer := time.NewTimer(time.Duration(seconds * float64(time.Second)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
