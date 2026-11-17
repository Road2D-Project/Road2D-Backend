package run

import (
	"Road-To-Destination-BE/module/tracking"
	"errors"
	"log"
	"strings"

	"Road-To-Destination-BE/middleware"
	authenController "Road-To-Destination-BE/module/authentication/controller"
	authenRepo "Road-To-Destination-BE/module/authentication/repository"
	authenService "Road-To-Destination-BE/module/authentication/service"
	friendController "Road-To-Destination-BE/module/friend/controller"
	groupController "Road-To-Destination-BE/module/group/controller"
	mapsClient "Road-To-Destination-BE/module/maps/client"
	mapsController "Road-To-Destination-BE/module/maps/controller"
	mapsService "Road-To-Destination-BE/module/maps/service"
	"Road-To-Destination-BE/module/realtime"
	realtimeController "Road-To-Destination-BE/module/realtime/controller"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/share/configuration"
	tripController "Road-To-Destination-BE/module/trip/controller"
	"Road-To-Destination-BE/utils/customValidator"

	"github.com/PeterTakahashi/gin-openapi/openapiui"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Wire dependencies
// Define controller
func Run() error {
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
	// Mount domain modules on this hub before the listener accepts sockets.
	// Redirect bypasses the leg cache while sharing provider quota with planning.
	hub := realtime.NewHub()
	redirectRouter := mapsService.NewRedirectService(goongClient)
	err = hub.Mount("tracking", tracking.NewTrackingModule(dbConfig.GetDatabase(), redirectRouter))
	if err != nil {
		log.Fatal(err)
	}
	routerRegistrars := []share.RouterRegistrar{
		authenController.NewAuthenticationController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, bucketLimiter),
		friendController.NewFriendController(dbConfig.GetDatabase(), authMw),
		groupController.NewGroupController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw),
		tripController.NewTripController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw, responseCacheMw, goongClient, bucketLimiter),
		realtimeController.NewLobbyController(authMw, hub, activeTripMemberAuthorizer{db: dbConfig.GetDatabase()}),
	}
	for _, r := range routerRegistrars {
		r.RegisterRoutes(v1)
	}
	addr := httpAddr()
	return router.Run(addr)
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
