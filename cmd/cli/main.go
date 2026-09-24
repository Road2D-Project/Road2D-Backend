package main

import (
	"log"
	"os"

	"Road-To-Destination-BE/cmd/cli/seed"
	"Road-To-Destination-BE/internal/seed/location"
)

func main() {
	if err := seed.Execute(location.RepairArgs(os.Args[1:])); err != nil {
		log.Fatal(err)
	}
}
