package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	evDisconnect     = "disconnect"
	evPause          = "pause"
	evDeviate        = "deviate"
	evWeakSignal     = "weakSignal"
	evLowBattery     = "lowBattery"
	evStayMode       = "stayMode"
	evUserStayPublic = "userStayPublic"
)

// Scenario is the mock file: one rider, one path, and timed events.
type Scenario struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Server      string   `json:"server"`
	RoomID      string   `json:"roomId"`
	Auth        Auth     `json:"auth"`
	Motion      Motion   `json:"motion"`
	Path        PathSpec `json:"path"`
	Events      []Event  `json:"events"`
}

type Auth struct {
	AccessToken string `json:"accessToken"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

type Motion struct {
	SpeedMps       float64 `json:"speedMps"`
	SendEveryMs    int     `json:"sendEveryMs"`
	BatteryPercent int     `json:"batteryPercent"`
	SignalDbm      int     `json:"signalDbm"`
	Loop           bool    `json:"loop"`
}

type PathSpec struct {
	Polyline string  `json:"polyline"`
	Points   []Point `json:"points"`
}

// Event is one scripted activity. At and Duration are seconds on the wall clock.
type Event struct {
	Name           string
	At             float64
	Duration       float64
	OffsetMeters   float64
	SignalDbm      *int
	BatteryPercent *int
	Note           string
}

func Load(path string) (Scenario, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, fmt.Errorf("không đọc được %s: %w", path, err)
	}
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	var sc Scenario
	if err := json.Unmarshal(body, &sc); err != nil {
		return Scenario{}, fmt.Errorf("JSON %s: %w", path, err)
	}
	return sc, nil
}

// UnmarshalJSON accepts either "events" or the shorter "event" key.
func (s *Scenario) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Server      string   `json:"server"`
		RoomID      string   `json:"roomId"`
		Auth        Auth     `json:"auth"`
		Motion      Motion   `json:"motion"`
		Path        PathSpec `json:"path"`
		Events      []Event  `json:"events"`
		Event       []Event  `json:"event"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Events) > 0 && len(raw.Event) > 0 {
		return errors.New("chỉ điền một trong hai khóa events hoặc event")
	}
	events := raw.Events
	if len(events) == 0 {
		events = raw.Event
	}
	*s = Scenario{
		Name:        raw.Name,
		Description: raw.Description,
		Server:      raw.Server,
		RoomID:      raw.RoomID,
		Auth:        raw.Auth,
		Motion:      raw.Motion,
		Path:        raw.Path,
		Events:      events,
	}
	return nil
}

func (e *Event) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name           string   `json:"name"`
		At             *float64 `json:"at"`
		AtSec          *float64 `json:"atSec"`
		Duration       *float64 `json:"duration"`
		DurationSec    *float64 `json:"durationSec"`
		OffsetMeters   float64  `json:"offsetMeters"`
		SignalDbm      *int     `json:"signalDbm"`
		BatteryPercent *int     `json:"batteryPercent"`
		Note           string   `json:"note"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	name, err := canonicalEvent(raw.Name)
	if err != nil {
		return err
	}
	at, err := oneFloat("at", raw.At, raw.AtSec)
	if err != nil {
		return err
	}
	dur, err := oneFloat("duration", raw.Duration, raw.DurationSec)
	if err != nil {
		return err
	}
	if at == nil {
		return errors.New("thiếu at (giây kích hoạt, tính từ lúc chạy)")
	}
	if dur == nil {
		return errors.New("thiếu duration (số giây hiệu lực)")
	}
	if *at < 0 {
		return fmt.Errorf("at phải >= 0, nhận %v", *at)
	}
	if *dur <= 0 {
		return fmt.Errorf("duration phải > 0, nhận %v", *dur)
	}
	*e = Event{
		Name:           name,
		At:             *at,
		Duration:       *dur,
		OffsetMeters:   raw.OffsetMeters,
		SignalDbm:      raw.SignalDbm,
		BatteryPercent: raw.BatteryPercent,
		Note:           strings.TrimSpace(raw.Note),
	}
	return nil
}

func oneFloat(label string, primary, alias *float64) (*float64, error) {
	if primary != nil && alias != nil && *primary != *alias {
		return nil, fmt.Errorf("%s và %sSec khác nhau", label, label)
	}
	if primary != nil {
		return primary, nil
	}
	return alias, nil
}

func canonicalEvent(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return "", errors.New("thiếu name")
	}
	if canonical, ok := eventAlias[key]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("name %q không hỗ trợ (%s)", name, knownEventList())
}

var eventOrder = []string{
	evDisconnect,
	evPause,
	evDeviate,
	evWeakSignal,
	evLowBattery,
	evStayMode,
	evUserStayPublic,
}

var eventAlias = map[string]string{
	"disconnect":     evDisconnect,
	"pause":          evPause,
	"deviate":        evDeviate,
	"weaksignal":     evWeakSignal,
	"lowbattery":     evLowBattery,
	"staymode":       evStayMode,
	"userstaypublic": evUserStayPublic,
}

func knownEventList() string {
	return strings.Join(eventOrder, ", ")
}

// Normalize fills defaults and checks ranges. Zero means "omitted" for speed, interval, battery, and signal.
func (s *Scenario) Normalize() error {
	s.Server = strings.TrimRight(strings.TrimSpace(s.Server), "/")
	if s.Server == "" {
		s.Server = "http://127.0.0.1:8080"
	}
	s.RoomID = strings.TrimSpace(s.RoomID)
	s.Auth.AccessToken = strings.TrimSpace(s.Auth.AccessToken)
	s.Auth.Username = strings.TrimSpace(s.Auth.Username)
	s.Name = strings.TrimSpace(s.Name)
	s.Description = strings.TrimSpace(s.Description)
	if s.Motion.SpeedMps == 0 {
		s.Motion.SpeedMps = 8
	}
	if s.Motion.SpeedMps <= 0 || s.Motion.SpeedMps > 80 {
		return fmt.Errorf("speedMps phải trong (0, 80], nhận %v", s.Motion.SpeedMps)
	}
	if s.Motion.SendEveryMs == 0 {
		s.Motion.SendEveryMs = 1000
	}
	if s.Motion.SendEveryMs < 200 {
		return fmt.Errorf("sendEveryMs tối thiểu 200, nhận %d", s.Motion.SendEveryMs)
	}
	if s.Motion.BatteryPercent == 0 {
		s.Motion.BatteryPercent = 80
	}
	if s.Motion.BatteryPercent < 0 || s.Motion.BatteryPercent > 100 {
		return errors.New("batteryPercent phải trong 1..100 (0 được hiểu là bỏ trống và thành 80)")
	}
	if s.Motion.SignalDbm == 0 {
		s.Motion.SignalDbm = -74
	}
	return nil
}

// Route uses points when there are at least two. Otherwise it decodes polyline.
func (s Scenario) Route() (Route, error) {
	if len(s.Path.Points) >= 2 {
		return BuildRoute(s.Path.Points, "points")
	}
	if strings.TrimSpace(s.Path.Polyline) != "" {
		points, err := Decode(s.Path.Polyline)
		if err != nil {
			return Route{}, err
		}
		return BuildRoute(points, "polyline")
	}
	return Route{}, errors.New("path cần polyline hoặc ít nhất 2 điểm trong points")
}

func describe(e Event) string {
	switch e.Name {
	case evDisconnect:
		return "đóng socket, đứng yên trên polyline"
	case evPause:
		return "đứng yên, socket vẫn mở"
	case evDeviate:
		meters := deviateMeters(e)
		if meters < 0 {
			return fmt.Sprintf("lệch trái %.0f m", -meters)
		}
		return fmt.Sprintf("lệch phải %.0f m", meters)
	case evWeakSignal:
		return fmt.Sprintf("signalDbm=%d", signalOf(e))
	case evLowBattery:
		return fmt.Sprintf("battery=%d%%", batteryOf(e))
	case evStayMode:
		return "gửi tracking.stayMode true, hết giờ gửi false"
	case evUserStayPublic:
		return "gửi tracking.userStayPublic true, hết giờ gửi false"
	default:
		return e.Name
	}
}

func deviateMeters(e Event) float64 {
	if e.OffsetMeters == 0 {
		return 80
	}
	return e.OffsetMeters
}

func signalOf(e Event) int {
	if e.SignalDbm != nil {
		return *e.SignalDbm
	}
	return -120
}

func batteryOf(e Event) int {
	if e.BatteryPercent != nil {
		return clampBattery(*e.BatteryPercent)
	}
	return 5
}

func clampBattery(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func authLabel(sc Scenario) string {
	if sc.Auth.AccessToken != "" {
		return "access token"
	}
	if sc.Auth.Username != "" {
		return "đăng nhập " + sc.Auth.Username
	}
	return "chưa cấu hình"
}
