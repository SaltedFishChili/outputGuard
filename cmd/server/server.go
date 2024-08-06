package main

import (
	"outputGuard/control"
)

func main() {
	server := control.NewControlServer()
	server.RunServer()
}
