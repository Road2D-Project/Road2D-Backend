package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
)

func main() {
	var file, server, room, token, username, password string
	var dry, quiet bool
	flag.StringVar(&file, "file", "", "file JSON kịch bản")
	flag.StringVar(&server, "server", "", "ghi đè server, ví dụ http://127.0.0.1:8080")
	flag.StringVar(&room, "room", "", "ghi đè roomId, UUID của trip")
	flag.StringVar(&token, "token", "", "access token, bỏ qua đăng nhập")
	flag.StringVar(&username, "username", "", "username nếu không có token")
	flag.StringVar(&password, "password", "", "password đăng nhập")
	flag.BoolVar(&dry, "dry-run", false, "in kịch bản và frame sẽ gửi, không mở socket")
	flag.BoolVar(&quiet, "quiet", false, "không in tracking.positions từ server")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Giả lập một rider trên websocket tracking.

  go run . -file ..\scenario.home.json -dry-run
  go run . -file ..\scenario.home.json -room <trip-uuid> -token <jwt>
  go run . -file ..\scenario.home.json -room <trip-uuid> -username demo -password secret

Cờ ghi đè file. Biến môi trường (khi cờ bỏ trống): MOCK_SERVER, MOCK_ROOM_ID,
MOCK_ACCESS_TOKEN, MOCK_USERNAME, MOCK_PASSWORD.

`)
		flag.PrintDefaults()
	}
	flag.Parse()
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(os.Stderr, "thiếu -file")
		flag.Usage()
		os.Exit(2)
	}

	sc, err := Load(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	applyOverrides(&sc, server, room, token, username, password)
	if err := sc.Normalize(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	route, err := sc.Route()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := Run(ctx, sc, route, options{Dry: dry, Quiet: quiet}, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func applyOverrides(sc *Scenario, server, room, token, username, password string) {
	sc.Server = firstNonEmpty(server, os.Getenv("MOCK_SERVER"), sc.Server)
	sc.RoomID = firstNonEmpty(room, os.Getenv("MOCK_ROOM_ID"), sc.RoomID)
	sc.Auth.AccessToken = firstNonEmpty(token, os.Getenv("MOCK_ACCESS_TOKEN"), sc.Auth.AccessToken)
	sc.Auth.Username = firstNonEmpty(username, os.Getenv("MOCK_USERNAME"), sc.Auth.Username)
	sc.Auth.Password = firstNonEmpty(password, os.Getenv("MOCK_PASSWORD"), sc.Auth.Password)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
