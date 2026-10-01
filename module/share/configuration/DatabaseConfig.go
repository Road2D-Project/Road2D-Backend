package configuration

import (
	"Road-To-Destination-BE/module/share"
	"context"
	"errors"
	"fmt"

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
	url := share.GetEnvStringDefault("DB_URL", "")
	if url == "" {
		return config.connectByVar()
	}
	return config.connectByURL(url)
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
func (config *DatabaseConfig) connectByURL(url string) error {
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	if err != nil {
		fmt.Println("postgres:", err)
	} else {
		sqlDB, err := db.DB()
		if err != nil {
			fmt.Println("postgres:", err)
			return err
		} else if err := sqlDB.PingContext(context.Background()); err != nil {
			fmt.Println("postgres:", err)
			return err
		}
	}
	config.db = db
	fmt.Println("Database connection established successfully")
	return nil
}
func (config *DatabaseConfig) connectByVar() error {
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
	return err
}
