package main

import (
	"outputGuard/control"
)

func main() {
	client := control.NewControlClient()
	go client.Exporter()
	go client.RecvierServerMessage()

	client.HandleIptablesMessage()
}
