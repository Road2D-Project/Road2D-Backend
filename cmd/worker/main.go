package main

import (
	"log"
	"os"

	"Road-To-Destination-BE/cmd/worker/run"
	"Road-To-Destination-BE/module/share"
)

func main() {
	if err := share.LoadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	if err := run.Run(); err != nil {
		log.Fatal(err)
	}
}
