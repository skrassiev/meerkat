package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/skrassiev/gsnowmelt_bot/sensor"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	var bot telega.Bot
	if err := bot.Init(interrupt, "process"); err != nil {
		log.Println(err)
		return
	}

	bot.AddHandler("/temp", sensor.HandleCommandlTemp)
	bot.AddHandler("/temp@gsnowmelt_bot", sensor.HandleCommandlTemp)
	bot.AddPeriodicTask(30*time.Minute, "Public IP Changed:", sensor.GetPublicIP)

	if status, err := bot.Run(); err == nil {
		log.Println(status)
	} else {
		log.Println("error", err)
	}
}
