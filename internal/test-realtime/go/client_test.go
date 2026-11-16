package main

import (
	"bytes"
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeMatchesTheKnownPolyline(t *testing.T) {
	points := []Point{
		{Lat: 38.5, Lng: -120.2},
		{Lat: 40.7, Lng: -120.95},
		{Lat: 43.252, Lng: -126.453},
	}
	const encoded = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
	if got := Encode(points); got != encoded {
		t.Fatalf("encode %q", got)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(points) {
		t.Fatalf("decoded %d points", len(decoded))
	}
	for i, point := range points {
		if math.Abs(decoded[i].Lat-point.Lat) > 1e-5 || math.Abs(decoded[i].Lng-point.Lng) > 1e-5 {
			t.Fatalf("point %d: got %+v", i, decoded[i])
		}
	}
}

func TestDisconnectAndPauseShareFrozenTime(t *testing.T) {
	events := []Event{
		{Name: evPause, At: 0, Duration: 10},
		{Name: evDisconnect, At: 5, Duration: 10},
	}
	if got := frozenSeconds(events, 12); math.Abs(got-12) > 1e-9 {
		t.Fatalf("overlap until 12 = %v", got)
	}
	if got := frozenSeconds(events, 20); math.Abs(got-15) > 1e-9 {
		t.Fatalf("union until 20 = %v", got)
	}
	motion := Motion{SpeedMps: 10, SendEveryMs: 1000, BatteryPercent: 80, SignalDbm: -74}
	along := alongMeters(motion, events, 20, 1000)
	if math.Abs(along-50) > 1e-6 {
		t.Fatalf("along %v, want 50", along)
	}
}

func TestDeviateMovesAboutTheOffset(t *testing.T) {
	route, err := BuildRoute([]Point{{Lat: 10.77, Lng: 106.70}, {Lat: 10.78, Lng: 106.70}}, "points")
	if err != nil {
		t.Fatal(err)
	}
	motion := Motion{SpeedMps: 8, SendEveryMs: 1000, BatteryPercent: 80, SignalDbm: -74}
	events := []Event{{Name: evDeviate, At: 0, Duration: 10, OffsetMeters: 80}}
	got := sampleAt(route, motion, events, 1)
	onLine, _ := route.at(got.AlongM)
	if dist := metersBetween(onLine, got.Point); dist < 70 || dist > 95 {
		t.Fatalf("offset distance %v, want about 80m", dist)
	}
	if got.Signal != -74 || got.Battery != 80 {
		t.Fatalf("battery/signal %+v", got)
	}
}

func TestWeakSignalAndLowBatteryOverrideTheSample(t *testing.T) {
	route, err := BuildRoute([]Point{{Lat: 10, Lng: 106}, {Lat: 10.01, Lng: 106}}, "points")
	if err != nil {
		t.Fatal(err)
	}
	signal := -118
	battery := 8
	motion := Motion{SpeedMps: 5, SendEveryMs: 1000, BatteryPercent: 90, SignalDbm: -70}
	events := []Event{
		{Name: evWeakSignal, At: 0, Duration: 5, SignalDbm: &signal},
		{Name: evLowBattery, At: 0, Duration: 5, BatteryPercent: &battery},
	}
	got := sampleAt(route, motion, events, 1)
	if got.Signal != -118 || got.Battery != 8 {
		t.Fatalf("signal %d battery %d", got.Signal, got.Battery)
	}
}

func TestEventAliasesAndUnknownName(t *testing.T) {
	const body = `{
		"server": "http://127.0.0.1:8080",
		"event": [{"name": "Disconnect", "atSec": 2, "durationSec": 4}]
	}`
	var sc Scenario
	if err := sc.UnmarshalJSON([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if len(sc.Events) != 1 || sc.Events[0].Name != evDisconnect || sc.Events[0].At != 2 || sc.Events[0].Duration != 4 {
		t.Fatalf("%+v", sc.Events)
	}
	var bad Scenario
	err := bad.UnmarshalJSON([]byte(`{"event":[{"name":"explode","at":1,"duration":1}]}`))
	if err == nil || !strings.Contains(err.Error(), "explode") {
		t.Fatalf("err %v", err)
	}
}

func TestPointsWinOverPolyline(t *testing.T) {
	sc := Scenario{
		Path: PathSpec{
			Polyline: "_p~iF~ps|U_ulLnnqC_mqNvxq`@",
			Points:   []Point{{Lat: 10, Lng: 106}, {Lat: 10.001, Lng: 106}},
		},
	}
	route, err := sc.Route()
	if err != nil {
		t.Fatal(err)
	}
	if route.Source != "points" || len(route.Points) != 2 {
		t.Fatalf("%s %d", route.Source, len(route.Points))
	}
}

func TestHorizonAddsFrozenTimeBack(t *testing.T) {
	route, err := BuildRoute([]Point{{Lat: 10, Lng: 106}, {Lat: 10.001, Lng: 106.001}}, "points")
	if err != nil {
		t.Fatal(err)
	}
	motion := Motion{SpeedMps: 10, SendEveryMs: 1000, BatteryPercent: 80, SignalDbm: -74}
	events := []Event{{Name: evDisconnect, At: 1, Duration: 4}}
	end := horizon(route, motion, events)
	moved := (end - frozenSeconds(events, end)) * motion.SpeedMps
	if math.Abs(moved-route.Length) > 0.05 {
		t.Fatalf("moved %v length %v end %v", moved, route.Length, end)
	}
	if end < 5 {
		t.Fatalf("end %v should include the disconnect", end)
	}
}

func TestNextMarkLandsOnAnEventEdge(t *testing.T) {
	events := []Event{{Name: evPause, At: 2.5, Duration: 1}}
	if got := nextMark(0, 1, events); math.Abs(got-1) > 1e-9 {
		t.Fatalf("first %v", got)
	}
	if got := nextMark(2, 1, events); math.Abs(got-2.5) > 1e-9 {
		t.Fatalf("edge %v", got)
	}
}

func TestDryRunDisconnectClosesAndReconnects(t *testing.T) {
	route, err := BuildRoute([]Point{{Lat: 10, Lng: 106}, {Lat: 10.002, Lng: 106}}, "points")
	if err != nil {
		t.Fatal(err)
	}
	sc := Scenario{
		Name:   "tiny",
		Server: "http://127.0.0.1:8080",
		RoomID: "11111111-1111-1111-1111-111111111111",
		Motion: Motion{SpeedMps: 10, SendEveryMs: 1000, BatteryPercent: 80, SignalDbm: -74},
		Events: []Event{{Name: evDisconnect, At: 1, Duration: 2}},
	}
	var buf bytes.Buffer
	if err := Run(context.Background(), sc, route, options{Dry: true}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"disconnect bật (2.0s)",
		"nối lại ws://127.0.0.1:8080/v1/realtime/rooms/11111111-1111-1111-1111-111111111111",
		"hết dry-run",
		`"type":"tracking.position"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q\n%s", want, out)
		}
	}
	if strings.Contains(sectionBetween(out, "disconnect bật", "disconnect tắt"), `"type":"tracking.position"`) {
		t.Fatalf("sent a position while disconnected\n%s", out)
	}
}

func sectionBetween(text, start, end string) string {
	from := strings.Index(text, start)
	if from < 0 {
		return ""
	}
	to := strings.Index(text[from:], end)
	if to < 0 {
		return text[from:]
	}
	return text[from : from+to]
}

func TestHomeScenarioDryRun(t *testing.T) {
	sc, err := Load(filepath.Join("..", "scenario.home.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sc.Normalize(); err != nil {
		t.Fatal(err)
	}
	route, err := sc.Route()
	if err != nil {
		t.Fatal(err)
	}
	if route.Source != "polyline" || len(route.Points) != len(homePoints) || len(sc.Events) != 7 {
		t.Fatalf("source %s points %d events %d", route.Source, len(route.Points), len(sc.Events))
	}
	if sc.Path.Polyline != Encode(homePoints) {
		t.Fatalf("polyline %q", sc.Path.Polyline)
	}
	for i, point := range homePoints {
		if math.Abs(route.Points[i].Lat-point.Lat) > 1e-5 || math.Abs(route.Points[i].Lng-point.Lng) > 1e-5 {
			t.Fatalf("point %d got %+v", i, route.Points[i])
		}
	}
	var buf bytes.Buffer
	if err := Run(context.Background(), sc, route, options{Dry: true}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"home-ride",
		"weakSignal bật",
		"disconnect bật (50.0s)",
		"pause bật",
		"lowBattery bật",
		"deviate bật",
		`"type":"tracking.stayMode","payload":true`,
		`"type":"tracking.stayMode","payload":false`,
		`"type":"tracking.userStayPublic","payload":true`,
		`"type":"tracking.userStayPublic","payload":false`,
		"hết dry-run",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

var homePoints = []Point{
	{Lat: 10.7725, Lng: 106.6980},
	{Lat: 10.7742, Lng: 106.7008},
	{Lat: 10.7761, Lng: 106.7034},
	{Lat: 10.7784, Lng: 106.7051},
	{Lat: 10.7809, Lng: 106.7076},
	{Lat: 10.7832, Lng: 106.7092},
}
