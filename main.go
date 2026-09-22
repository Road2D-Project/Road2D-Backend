package main

import (
	"Road-To-Destination-BE/utils/customValidator"
	"errors"
	"log"
	"os"

	"Road-To-Destination-BE/middleware"
	authenController "Road-To-Destination-BE/module/authentication/controller"
	authenRepo "Road-To-Destination-BE/module/authentication/repository"
	groupController "Road-To-Destination-BE/module/group/controller"
	mapsClient "Road-To-Destination-BE/module/maps/client"
	mapsController "Road-To-Destination-BE/module/maps/controller"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/share/configuration"
	tripController "Road-To-Destination-BE/module/trip/controller"

	"github.com/PeterTakahashi/gin-openapi/openapiui"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

//go:generate swag init -g main.go -d . --exclude ./docs -o ./docs --parseInternal --outputTypes json,yaml

// @title           Road2D
// @version         1.0
// @description     Backend API for Road2D (team Vandra): collaborative multi-branch ride planning and group-ride safety in Vietnam. JWT auth, Goong Maps playground, trip graph models.
// @BasePath        /v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey PlaygroundKey
// @in header
// @name X-Playground-Key
// @description Shared secret from GOONG_PLAYGROUND_KEY. Scalar → Authorize.
func main() {
	if err := share.LoadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	dbConfig := configuration.DatabaseConfig{}
	if err := dbConfig.ConnectDatabase(); err != nil {
		if errors.Is(err, configuration.ErrDatabaseNotConfigured) {
			log.Print("postgres skipped: set DB_HOST, DB_USER, DB_NAME, DB_PORT to enable")
		} else {
			log.Fatal(err)
		}
	} else if err := configuration.AutoMigrate(dbConfig.GetDatabase()); err != nil {
		log.Fatal(err)
	}

	var redisConfig configuration.RedisConfiguration
	redisConfig.Connect()
	defer redisConfig.Disconnect()
	// validator
	mainValidator := validator.New()
	customValidator.ValidatorRegistrar(mainValidator)

	router := gin.Default()

	router.GET("/docs/public/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/docs/public/openapi.json",
		SpecFilePath: "./docs/swagger.json",
		Title:        "Road2D",
		Theme:        "dark",
	}))

	v1 := router.Group("/v1")
	// Playgroup router
	mapsClient.NewSerpClient().RegisterRoutes(v1)
	playground := v1.Group("")
	playground.Use(middleware.RequireHeaderPlaygroundKey())
	sandboxes := []share.PlaygroundRegistrar{
		mapsController.NewMapController(redisConfig.Client()),
	}
	for _, r := range sandboxes {
		r.RegisterPlayground(playground)
	}
	// Public / authenticated routers
	authMw := middleware.NewAuthenticationMiddleware(
		authenRepo.NewCacheUserRepository(dbConfig.GetDatabase(), redisConfig.Client()),
	)
	routerRegistrars := []share.RouterRegistrar{
		authenController.NewAuthenticationController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator),
		groupController.NewGroupController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw),
		tripController.NewTripController(dbConfig.GetDatabase(), redisConfig.Client(), mainValidator, authMw),
	}
	for _, r := range routerRegistrars {
		r.RegisterRoutes(v1)
	}

	addr := share.GetEnvStringDefault("HTTP_ADDR", ":8080")
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
