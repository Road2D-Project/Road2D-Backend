package run

import (
	"errors"
	"log"
	"strings"

	"Road-To-Destination-BE/middleware"
	authenController "Road-To-Destination-BE/module/authentication/controller"
	authenRepo "Road-To-Destination-BE/module/authentication/repository"
	groupController "Road-To-Destination-BE/module/group/controller"
	mapsClient "Road-To-Destination-BE/module/maps/client"
	mapsController "Road-To-Destination-BE/module/maps/controller"
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
	dbConfig := configuration.DatabaseConfig{}
	var redisConfig configuration.RedisConfiguration
	redisConfig.Connect()
	if err := dbConfig.ConnectDatabase(); err != nil {
		if errors.Is(err, configuration.ErrDatabaseNotConfigured) {
			log.Print("postgres skipped: set DB_HOST, DB_USER, DB_NAME, DB_PORT to enable")
		} else {
			return err
		}
	} /*else if err := configuration.AutoMigrate(dbConfig.GetDatabase()); err != nil {
		return err
	}*/
	defer redisConfig.Disconnect()

	mainValidator := validator.New()
	customValidator.ValidatorRegistrar(mainValidator)

	router := gin.Default()
	registerHealth(router, dbConfig.GetDatabase(), redisConfig.Client())
	router.GET("/docs/public/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/docs/public/openapi.json",
		SpecFilePath: "./docs/swagger.json",
		Title:        "Road2D",
		Theme:        "dark",
	}))

	v1 := router.Group("/v1")
	mapsClient.NewSerpClient().RegisterRoutes(v1)
	playground := v1.Group("")
	playground.Use(middleware.RequireHeaderPlaygroundKey())
	sandboxes := []share.PlaygroundRegistrar{
		mapsController.NewMapController(dbConfig.GetDatabase(), redisConfig.Client()),
	}
	for _, r := range sandboxes {
		r.RegisterPlayground(playground)
	}

	authMw := middleware.NewAuthenticationMiddleware(
		authenRepo.NewCacheUserRepository(dbConfig.GetDatabase(), redisConfig.Client()),
	)
	responseCacheMw := middleware.NewResponseCache(redisConfig.Client())
	routerRegistrars := []share.RouterRegistrar{
		authenController.NewAuthenticationController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator),
		groupController.NewGroupController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw),
		tripController.NewTripController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw, responseCacheMw),
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
