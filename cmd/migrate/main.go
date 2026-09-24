package main

import (
	"log"

	"Road-To-Destination-BE/cmd/migrate/run"
)

func main() {
	if err := run.Run(); err != nil {
		log.Fatal(err)
	}
}
