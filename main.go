package main

import "Road-To-Destination-BE/cmd"

//go:generate swag init -g cmd/server/main.go -d . --exclude ./docs -o ./docs --parseInternal --outputTypes json,yaml

func main() {
	cmd.Execute()
}
