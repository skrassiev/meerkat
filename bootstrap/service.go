package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"os/signal"
	"syscall"

	"github.com/skrassiev/gsnowmelt_bot/feed"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

type ServiceMode byte

const (
	ServiceModeNone     = 0
	ServiceModeCommands = 1 << iota
	ServiceModePeriodic
	ServiceModeFSMoinitor
	ServiceModeHealthcheck
)

// Main adds standard handlers to the telega bot.
func Main(runtime string, serviceMode byte) (status string, err error) {

	ctx, cancel := context.WithCancel(context.Background())

	var bot telega.Bot
	if err = bot.Init(ctx, runtime); err != nil {
		log.Println(err)
		cancel()
		return "failed to init", err
	}

	log.Println("telegram API initialized")

	if (serviceMode & ServiceModeCommands) == ServiceModeCommands {
		// add handlers
		log.Println("adding commands handlers")
		bot.AddHandler("/temp", feed.HandleCommandlTemp)
		if imageURL := os.Getenv("IMAGE_URL"); len(imageURL) > 0 {
			bot.AddHandler("/pic", feed.GetPictureByURL(imageURL))
		}
	}

	if (serviceMode & ServiceModePeriodic) == ServiceModePeriodic {
		// add periodic tasks
		log.Println("adding periodic tasks handlers")
		bot.AddPeriodicTask(30*time.Minute, "Public IP Changed:", feed.PublicIP)
	}

	if (serviceMode & ServiceModeFSMoinitor) == ServiceModeFSMoinitor {
		// add FS monitor
		log.Println("adding background tasks")
		directores := os.Getenv("MONITORED_DIRECTORIES")
		if len(strings.TrimSpace(directores)) > 0 {
			for _, v := range strings.Split(strings.TrimSpace(directores), ";") {
				if fs.ValidPath(v) {
					bot.AddBackgroundTask(feed.MonitorDirectoryTree(v, feed.NewfileFilterChain(feed.FilenameFilter([]string{`(?i)\.jpg$`}))))
				} else {
					log.Println("fsmonitor: invalid path", v)
					bot.AddBackgroundTask(feed.MonitorDirectoryTree(v, feed.NewfileFilterChain(feed.FilenameFilter([]string{`(?i)\.jpg$`}))))
				}
			}
		}
	}

	if (serviceMode & ServiceModeHealthcheck) == ServiceModeHealthcheck {
		bot.AddHandler("/ping", feed.PingCommand)
	}

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
