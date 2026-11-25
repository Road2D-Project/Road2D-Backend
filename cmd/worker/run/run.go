package run

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	mapsClient "Road-To-Destination-BE/module/maps/client"
	mapsService "Road-To-Destination-BE/module/maps/service"
	"Road-To-Destination-BE/module/share/configuration"
	"Road-To-Destination-BE/module/tracking/repository"
	"Road-To-Destination-BE/module/tracking/worker"
)

func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var database configuration.DatabaseConfig
	if err := database.ConnectDatabase(); err != nil {
		return err
	}
	sqlDB, err := database.GetDatabase().DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var redisConfig configuration.RedisConfiguration
	client := redisConfig.Connect()
	if client == nil {
		return errors.New("tracking worker requires Redis")
	}
	defer redisConfig.Disconnect()
	quota, err := mapsClient.NewProviderQuota("goong", client)
	if err != nil {
		return err
	}
	router := mapsService.NewRedirectService(mapsClient.NewGoongClient(quota))
	results := repository.NewRedirectResultRepository(database.GetDatabase())
	worker.NewRedirectWorker(client, router, results).Run(ctx)
	return nil
}
