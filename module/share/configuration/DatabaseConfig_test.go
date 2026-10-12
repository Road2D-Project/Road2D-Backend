package configuration

import "testing"

func TestPostgresPort(t *testing.T) {
	cases := []struct {
		dsn  string
		port string
	}{
		{dsn: "postgresql://user:secret@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres", port: "6543"},
		{dsn: "postgres://user:secret@db.example.supabase.co:5432/postgres", port: "5432"},
		{dsn: "host=localhost user=postgres password=secret dbname=app port=5432 sslmode=disable", port: "5432"},
		{dsn: "host=localhost user=postgres dbname=app port=6543", port: "6543"},
		{dsn: "host=localhost user=postgres dbname=app", port: ""},
	}
	for _, tc := range cases {
		if got := postgresPort(tc.dsn); got != tc.port {
			t.Fatalf("postgresPort(%q) = %q, want %q", tc.dsn, got, tc.port)
		}
	}
}
