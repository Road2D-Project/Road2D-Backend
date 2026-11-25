// Package testutil provides isolated infrastructure for opt-in integration tests.
package testutil

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Postgres owns one disposable schema, never an application's existing tables.
// Callers choose which migrations to run so legacy-data backfills can be tested.
func Postgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ROAD2D_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ROAD2D_TEST_POSTGRES_DSN to run PostgreSQL integration tests")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid PostgreSQL test configuration")
	}
	config.ConnectTimeout = 5 * time.Second
	admin := stdlib.OpenDB(*config)
	schemaName := "chat_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.ExecContext(t.Context(), `CREATE SCHEMA "`+schemaName+`"`); err != nil {
		admin.Close()
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP SCHEMA "` + schemaName + `" CASCADE`); err != nil {
			t.Errorf("remove test schema: %v", err)
		}
		admin.Close()
	})
	if config.RuntimeParams == nil {
		config.RuntimeParams = map[string]string{}
	}
	config.RuntimeParams["search_path"] = schemaName
	connection := stdlib.OpenDB(*config)
	t.Cleanup(func() { connection.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}
