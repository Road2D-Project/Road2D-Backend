package main

import (
	"errors"
	"log"
	"os"

	"Road-To-Destination-BE/middleware"
	mapsController "Road-To-Destination-BE/module/maps/controller"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/share/configuration"
	tripModel "Road-To-Destination-BE/module/trip/model"

	"github.com/PeterTakahashi/gin-openapi/openapiui"
	"github.com/gin-gonic/gin"
)

//go:generate swag init -g main.go -d . --exclude ./docs -o ./docs --parseInternal --outputTypes json,yaml

// @title           Road2D
// @version         1.0
// @description     API Server for Road2D
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
	} else if err := tripModel.AutoMigrate(dbConfig.GetDatabase()); err != nil {
		log.Fatal(err)
	}

	var redisConfig configuration.RedisConfiguration
	redisConfig.Connect()
	defer redisConfig.Disconnect()

	router := gin.Default()
	router.GET("/docs/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/docs/openapi.json",
		SpecFilePath: "./docs/swagger.json",
		Title:        "Road2D",
		Theme:        "dark",
	}))

	v1 := router.Group("/v1")
	playground := v1.Group("")
	playground.Use(middleware.RequireHeaderPlaygroundKey())
	sandboxes := []share.PlaygroundRegistrar{
		mapsController.NewMapController(),
	}
	for _, r := range sandboxes {
		r.RegisterPlayground(playground)
	}

	addr := share.GetEnvStringDefault("HTTP_ADDR", ":8080")
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
