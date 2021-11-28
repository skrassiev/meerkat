package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/skrassiev/gsnowmelt_bot/sensor"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	status, err := sensor.ServeBotAPI(interrupt, "process")
	if err == nil {
		log.Println(status)
	} else {
		log.Println("error", err)
	}
}
