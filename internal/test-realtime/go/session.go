package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// roomURL is the lobby socket. Native clients send the JWT on the Authorization header.
func roomURL(server, roomID string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(server))
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("server không hợp lệ: %s", server)
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("server cần http, https, ws hoặc wss: %s", server)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/realtime/rooms/" + url.PathEscape(roomID)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func requireRoom(roomID string) error {
	if roomID == "" {
		return errors.New("cần roomId (cờ -room hoặc field roomId), là UUID của trip")
	}
	if !uuidPattern.MatchString(roomID) {
		return fmt.Errorf("roomId phải là UUID của trip, nhận %q", roomID)
	}
	return nil
}

func login(ctx context.Context, server, username, password string) (string, error) {
	if username == "" || password == "" {
		return "", errors.New("cần accessToken hoặc cả username và password")
	}
	payload, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(server, "/") + "/v1/auth/user/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("đăng nhập: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("đăng nhập %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("đăng nhập trả body không đọc được: %s", strings.TrimSpace(string(body)))
	}
	if parsed.Data.AccessToken == "" {
		return "", errors.New("đăng nhập không trả access_token")
	}
	return parsed.Data.AccessToken, nil
}

func dialSocket(ctx context.Context, endpoint, token string) (*websocket.Conn, error) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	conn, resp, err := dialer.DialContext(ctx, endpoint, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			text := strings.TrimSpace(string(body))
			if text == "" {
				text = resp.Status
			}
			return nil, fmt.Errorf("websocket %s: %s", resp.Status, text)
		}
		return nil, fmt.Errorf("websocket: %w", err)
	}
	return conn, nil
}

// link is one live socket. The runner writes; readPump is the only reader.
type link struct {
	conn       *websocket.Conn
	quietClose atomic.Bool
	dead       atomic.Bool
}

func (l *link) send(frame []byte) error {
	if l == nil || l.dead.Load() {
		return errors.New("socket chưa mở")
	}
	_ = l.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return l.conn.WriteMessage(websocket.TextMessage, frame)
}

func (l *link) close() {
	if l == nil {
		return
	}
	l.quietClose.Store(true)
	_ = l.conn.Close()
}

func (l *link) readPump(out *printer, start time.Time, quiet bool) {
	defer l.dead.Store(true)
	l.conn.SetReadLimit(1 << 20)
	for {
		_, msg, err := l.conn.ReadMessage()
		if err != nil {
			if !l.quietClose.Load() {
				out.timed(time.Since(start).Seconds(), "recv", "socket đóng: "+err.Error())
			}
			return
		}
		if quiet && messageType(msg) == "tracking.positions" {
			continue
		}
		out.timed(time.Since(start).Seconds(), "recv", compactJSON(msg))
	}
}

func messageType(raw []byte) string {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return ""
	}
	return head.Type
}

func compactJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func positionFrame(point Point, battery, signal int) string {
	return fmt.Sprintf(
		`{"type":"tracking.position","payload":{"lat":%.6f,"lng":%.6f,"batteryPercent":%d,"signalDbm":%d}}`,
		point.Lat, point.Lng, battery, signal,
	)
}

func boolFrame(msgType string, value bool) string {
	return fmt.Sprintf(`{"type":%q,"payload":%t}`, msgType, value)
}
