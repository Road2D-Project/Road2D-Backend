package run

import (
	"os"

	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/share/configuration"
)

func Run() error {
	if err := share.LoadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
		return err
	}
	var dbConfig configuration.DatabaseConfig
	if err := dbConfig.ConnectDatabase(); err != nil {
		return err
	}
	return configuration.AutoMigrate(dbConfig.GetDatabase())
}
