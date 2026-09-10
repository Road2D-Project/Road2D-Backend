package main

import (
	"log"
	"os"

	mapsController "Road-To-Destination-BE/module/maps/controller"
	"Road-To-Destination-BE/module/share"

	openapiui "github.com/PeterTakahashi/gin-openapi/openapiui"
	"github.com/gin-gonic/gin"
)

//go:generate swag init -g main.go -o ./docs --parseInternal

// @title           Road To Destination API Document
// @version         1.0
// @description     API Server for GWYG Application
// @BasePath        /v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	if err := share.LoadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	router := gin.Default()
	router.GET("/docs/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/docs/openapi.json",
		SpecFilePath: "./docs/swagger.json",
		Title:        "Road To Destination",
		Theme:        "dark",
	}))

	v1 := router.Group("/v1")
	routers := []share.RouterRegistrar{
		mapsController.NewMapController(),
	}
	for _, r := range routers {
		r.RegisterRoutes(v1)
	}

	addr := share.GetEnvStringDefault("HTTP_ADDR", ":8080")
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
