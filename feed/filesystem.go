package feed

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

func MonitorDirectoryTree(directory string, recursive bool) telega.BackgroundFunction {
	// we should always use a new instance of the watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	return func(ctx context.Context, events chan<- telega.ChattableCloser) {
		defer watcher.Close()
		// track directories
		tracking := make(map[string]struct{})
		done := make(chan bool)

		// only when this function is launched, we should start monitoring the FS to avoid losing the events.

		go func() {
			defer close(done)
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					log.Println("event:", event)
					if fname := onFsModification(event); len(fname) != 0 {
						if tgEvent, err := processFile(fname); err != nil {
							log.Println("eror handling file", fname)
						} else {
							events <- tgEvent
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("error:", err)
				case <-ctx.Done():
					return
				}
			}
		}()

		rootFS := os.DirFS(directory)
		fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Fatal(err)
			}
			fi, err := os.Stat(path)
			if err != nil {
				log.Fatal(err)
			}
			if fi.IsDir() {
				if err = watcher.Add(path); err != nil {
					log.Fatal(err)
				}
				tracking[path] = struct{}{}
			}

			fmt.Println(path)
			return nil
		})

		<-done
	}
}

func onFsModification(event fsnotify.Event, watcher *fsnotify.Watcher, tracking map[string]struct{}) (newFileName string) {
	if event.Op == fsnotify.Rename || event.Op == fsnotify.Chmod {
		// speaking of motion, it will not rename files. Also, chmod-ing is not expected as well.
		return
	}
	fi, err := os.Stat(event.Name)
	if err != nil {
		log.Println("onFsModification", event)
		return
	}
	if fi.IsDir() {
		_, exist := tracking[event.Name]
		switch event.Op {
		case fsnotify.Create:
			if !exist {
				if err := watcher.Add(event.Name); err != nil {
					log.Println("failed to add directory", event.Name, "watch:", err)
					return
				}
				tracking[event.Name] = struct{}{}
			}
		case fsnotify.Remove:
			if exist {
				watcher.Remove(event.Name)
				delete(tracking, event.Name)
			}
		}
	} else {
		// it's a file
		if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
			log.Println("modified or created file:", event.Name)
			return event.Name
		}
	}
	return
}

func processFile(fname string) (telega.ChattableCloser, error) {
	return telega.ChattableText{
		Chattable: tgbotapi.NewMessage(0, fmt.Sprintf("file %s added", fname)),
	}, nil
}
