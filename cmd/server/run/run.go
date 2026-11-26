package run

import (
	"Road-To-Destination-BE/module/tracking"
	trackingRepository "Road-To-Destination-BE/module/tracking/repository"
	trackingWorker "Road-To-Destination-BE/module/tracking/worker"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"Road-To-Destination-BE/middleware"
	authenController "Road-To-Destination-BE/module/authentication/controller"
	authenRepo "Road-To-Destination-BE/module/authentication/repository"
	authenService "Road-To-Destination-BE/module/authentication/service"
	conversationController "Road-To-Destination-BE/module/conversation/controller"
	friendController "Road-To-Destination-BE/module/friend/controller"
	groupController "Road-To-Destination-BE/module/group/controller"
	mapsClient "Road-To-Destination-BE/module/maps/client"
	mapsController "Road-To-Destination-BE/module/maps/controller"
	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/realtime"
	realtimeController "Road-To-Destination-BE/module/realtime/controller"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/share/configuration"
	tripController "Road-To-Destination-BE/module/trip/controller"
	"Road-To-Destination-BE/utils/customValidator"

	"github.com/PeterTakahashi/gin-openapi/openapiui"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Wire dependencies
// Define controller
func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := authenService.ValidateJWTSecrets(); err != nil {
		return err
	}
	dbConfig := configuration.DatabaseConfig{}
	var redisConfig configuration.RedisConfiguration
	redisConfig.Connect()
	if err := dbConfig.ConnectDatabase(); err != nil {
		if errors.Is(err, configuration.ErrDatabaseNotConfigured) {
			log.Print("postgres skipped: set DB_HOST, DB_USER, DB_NAME, DB_PORT to enable")
		} else {
			return err
		}
	}
	defer redisConfig.Disconnect()

	mainValidator := validator.New()
	customValidator.ValidatorRegistrar(mainValidator)

	router := gin.New()
	//lift access token to avoid sensitive log message
	router.Use(middleware.LiftAccessToken(), middleware.RequestLogger(), gin.Recovery())
	registerHealth(router, dbConfig.GetDatabase(), redisConfig.Client())
	router.GET("/docs/public/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/docs/public/openapi.json",
		SpecFilePath: "./docs/swagger.json",
		Title:        "Road2D",
		Theme:        "dark",
	}))

	v1 := router.Group("/v1")

	playground := v1.Group("")
	playground.Use(middleware.RequireHeaderPlaygroundKey())
	bucketLimiter, bucketError := middleware.NewTokenBucket(middleware.DefaultTokenBucketConfig(), redisConfig.Client())
	if bucketError != nil {
		return bucketError
	}
	goongQuota, err := mapsClient.NewProviderQuota("goong", redisConfig.Client())
	if err != nil {
		return err
	}
	serpQuota, err := mapsClient.NewProviderQuota("serp", redisConfig.Client())
	if err != nil {
		return err
	}
	serpClient, err := mapsClient.NewSerpClient(bucketLimiter, serpQuota)
	if err != nil {
		return err
	}
	serpClient.RegisterRoutes(playground)
	goongClient := mapsClient.NewGoongClient(goongQuota)
	sandboxes := []share.PlaygroundRegistrar{
		mapsController.NewMapController(dbConfig.GetDatabase(), redisConfig.Client(), goongClient, bucketLimiter),
	}
	for _, r := range sandboxes {
		r.RegisterPlayground(playground)
	}

	authMw := middleware.NewAuthenticationMiddleware(
		authenRepo.NewUserRepository(dbConfig.GetDatabase()),
	)
	responseCacheMw := middleware.NewResponseCache(redisConfig.Client())
	// One server owns a trip's coordinator. Distributed workers return results
	// through this process's owner-specific stream.
	hub := realtime.NewHub()
	ownerID := uuid.New()
	checkpointRepository := trackingRepository.NewCheckpointRepository(dbConfig.GetDatabase())
	checkpointWriter := trackingWorker.NewCheckpointWriter(checkpointRepository)
	redirectPublisher := trackingWorker.NewRedirectPublisher(redisConfig.Client(), ownerID)
	redirectResults := trackingWorker.NewRedirectResultConsumer(redisConfig.Client(), ownerID)
	checkpointInterval := share.GetEnvIntDefault("TRACKING_CHECKPOINT_INTERVAL_SECONDS", 5)
	if checkpointInterval <= 0 {
		return errors.New("TRACKING_CHECKPOINT_INTERVAL_SECONDS must be positive")
	}
	err = hub.Mount("tracking", tracking.NewTrackingModuleWithRecovery(dbConfig.GetDatabase(), tracking.RecoveryDependencies{
		Checkpoints: checkpointRepository, Writer: checkpointWriter,
		Redirects: redirectPublisher, Results: redirectResults,
		CheckpointInterval: time.Duration(checkpointInterval) * time.Second,
	}))
	if err != nil {
		return err
	}
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	for _, run := range []func(context.Context){checkpointWriter.Run, redirectPublisher.Run, redirectResults.Run} {
		workers.Go(func() { run(workerCtx) })
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := hub.Close(closeCtx); err != nil {
			log.Printf("close tracking rooms: %v", err)
		}
		stopWorkers()
		workers.Wait()
		if err := checkpointWriter.Flush(closeCtx); err != nil {
			log.Printf("flush tracking checkpoints: %v", err)
		}
	}()
	mediaStorage := objectStorage.NewClient()
	routerRegistrars := []share.RouterRegistrar{
		conversationController.NewChatController(dbConfig.GetDatabase(), authMw, bucketLimiter),
		authenController.NewAuthenticationController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, bucketLimiter, mediaStorage),
		friendController.NewFriendController(dbConfig.GetDatabase(), authMw),
		groupController.NewGroupController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw),
		tripController.NewTripController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw, responseCacheMw, goongClient, bucketLimiter, mediaStorage),
		realtimeController.NewLobbyController(authMw, hub, activeTripMemberAuthorizer{db: dbConfig.GetDatabase()}),
	}
	for _, r := range routerRegistrars {
		r.RegisterRoutes(v1)
	}
	addr := httpAddr()
	server := &http.Server{Addr: addr, Handler: router, ReadHeaderTimeout: 10 * time.Second}
	serveDone := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Printf("shutdown HTTP server: %v", err)
				_ = server.Close()
			}
		case <-serveDone:
		}
	}()
	err = server.ListenAndServe()
	close(serveDone)
	<-shutdownDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func httpAddr() string {
	if addr := share.GetEnvStringDefault("HTTP_ADDR", ""); addr != "" {
		return listenAddr(addr)
	}
	if port := share.GetEnvStringDefault("PORT", ""); port != "" {
		return listenAddr(port)
	}
	return ":8080"
}

func listenAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	return ":" + addr
}
