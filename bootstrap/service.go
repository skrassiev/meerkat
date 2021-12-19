package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"os/signal"
	"syscall"

	"github.com/skrassiev/gsnowmelt_bot/feed"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

// Main adds standard handlers to the telega bot.
func Main(runtime string) (status string, err error) {

	ctx, cancel := context.WithCancel(context.Background())

	var bot telega.Bot
	if err = bot.Init(ctx, runtime); err != nil {
		log.Println(err)
		cancel()
		return "failed to init", err
	}

	// add handlers
	bot.AddHandler("/temp", feed.HandleCommandlTemp)
	if imageURL := os.Getenv("IMAGE_URL"); len(imageURL) > 0 {
		bot.AddHandler("/pic", feed.GetPictureByURL(imageURL))
	}

	//  add periodic tasks
	bot.AddPeriodicTask(30*time.Minute, "Public IP Changed:", feed.PublicIP)

	// synchronization tasks
	var wg sync.WaitGroup

	// run the bot in a waitgroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		status, err = bot.Run()
	}()

	// finish on a potential Bot failure as well
	done := make(chan struct{})
	go func() {
		wg.Wait()
		log.Println("wg finished")
		done <- struct{}{}
	}()

	// process management
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-interrupt:
			cancel()
			log.Printf("%s was interrupted by system signal", runtime)
			time.Sleep(1 * time.Second)
			return fmt.Sprintf("%s was interrupted by system signal", runtime), nil
		case <-done:
			cancel()
			if err == nil {
				log.Println(status)
			} else {
				log.Println("error", err)
			}
			return
		}
	}

}
