package configuration

import (
	"errors"
	"fmt"

	"Road-To-Destination-BE/module/share"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrDatabaseNotConfigured = errors.New("postgres is not configured (set DB_HOST, DB_USER, DB_NAME, DB_PORT)")

// DatabaseConfig mirrors basic_restful, but DSN always comes from env — never a hardcoded password.
type DatabaseConfig struct {
	db  *gorm.DB
	dsn string
}

func (config *DatabaseConfig) ConnectDatabase() error {
	host := share.GetEnvStringDefault("DB_HOST", "")
	user := share.GetEnvStringDefault("DB_USER", "")
	password := share.GetEnvStringDefault("DB_PASSWORD", "")
	dbname := share.GetEnvStringDefault("DB_NAME", "")
	port := share.GetEnvStringDefault("DB_PORT", "")
	if host == "" || user == "" || dbname == "" || port == "" {
		return ErrDatabaseNotConfigured
	}

	sslmode := share.GetEnvStringDefault("DB_SSLMODE", "disable")
	tz := share.GetEnvStringDefault("DB_TIMEZONE", "Asia/Singapore")
	config.dsn = fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, user, password, dbname, port, sslmode, tz,
	)

	database, err := gorm.Open(postgres.Open(config.dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	config.db = database
	return nil
}

func (config *DatabaseConfig) Migrate(models ...interface{}) error {
	if config.db == nil {
		return ErrDatabaseNotConfigured
	}
	return config.db.AutoMigrate(models...)
}

func (config *DatabaseConfig) GetDatabase() *gorm.DB {
	return config.db
}
