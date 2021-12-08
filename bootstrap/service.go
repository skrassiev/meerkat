package bootstrap

import (
	"log"
	"os"
	"time"

	"os/signal"
	"syscall"

	"github.com/skrassiev/gsnowmelt_bot/feed"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

// Main adds standard handlers to the telega bot
func Main(runtime string) (status string, err error) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	var bot telega.Bot
	if err = bot.Init(interrupt, runtime); err != nil {
		log.Println(err)
		return "failed to init", err
	}

	bot.AddHandler("/temp", feed.HandleCommandlTemp)
	bot.AddHandler("/temp@gsnowmelt_bot", feed.HandleCommandlTemp)
	if imageURL := os.Getenv("IMAGE_URL"); len(imageURL) > 0 {
		bot.AddHandler("/pic", feed.GetPictureByURL(imageURL))
	}

	bot.AddPeriodicTask(30*time.Minute, "Public IP Changed:", feed.PublicIP)

	status, err = bot.Run()
	if err == nil {
		log.Println(status)
	} else {
		log.Println("error", err)
	}
	return status, err
}
