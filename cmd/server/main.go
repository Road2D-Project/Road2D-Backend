package main

import (
	"log"
	"os"

	"Road-To-Destination-BE/cmd/server/run"
	"Road-To-Destination-BE/module/share"
)

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
	if err := run.Run(); err != nil {
		log.Fatal(err)
	}
}
