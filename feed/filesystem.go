package feed

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

var (
	fsw *fsnotify.Watcher
)

func MonitorDirectoryTree(directory string, recursive bool, monitored *int32) telega.BackgroundFunction {
	// we should always use a new instance of the watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	fsw = watcher
	if monitored == nil {
		monitored = new(int32)
	}
	return func(ctx context.Context, events chan<- telega.ChattableCloser) {
		defer watcher.Close()
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
					if fname := onFsModification(event, watcher, monitored); len(fname) != 0 {
						log.Printf("%+v\n", *watcher)
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
					log.Printf("%+v\n", *watcher)
					return
				}
			}
		}()

		rootFS := os.DirFS(directory)
		fs.WalkDir(rootFS, ".", walkFunction(directory, func(fpath string) error {
			atomic.AddInt32(monitored, 1)
			return watcher.Add(fpath)
		}))

		<-done
	}
}

func onFsModification(event fsnotify.Event, watcher *fsnotify.Watcher, monitored *int32) (newFileName string) {
	if event.Op == fsnotify.Rename || event.Op == fsnotify.Chmod {
		// speaking of motion, it will not rename files. Also, chmod-ing is not expected as well.
		return
	}

	if (event.Op & fsnotify.Create) != 0 {
		fi, err := os.Stat(event.Name)
		if err != nil {
			log.Println("onFsModification", event, err)
			return
		}
		if fi.IsDir() {
			if err := watcher.Add(event.Name); err != nil {
				log.Println("failed to add directory", event.Name, "watch:", err)
				return
			} else {
				log.Println("added", event.Name)
			}
			atomic.AddInt32(monitored, 1)
		} else if (event.Op & fsnotify.Write) != 0 {
			// it's a newly created file
			log.Println("modified or created file:", event.Name)
			return event.Name
		}
	} else if (event.Op & fsnotify.Remove) != 0 {
		// We don't know if it was a dir or file.
		// On top of that, it has alredy been removed by fsnotify.
		// So we do best effort
		//		log.Println("removing", *monitored)
		atomic.AddInt32(monitored, -1)
	}

	return
}

func processFile(fname string) (telega.ChattableCloser, error) {
	return telega.ChattableText{
		Chattable: tgbotapi.NewMessage(0, fmt.Sprintf("file %s added", fname)),
	}, nil
}

func walkFunction(rootPath string, onDirectory func(fpath string) error) func(fpath string, d fs.DirEntry, err error) error {
	return func(fpath string, d fs.DirEntry, err error) error {
		fpath = path.Join(rootPath, fpath)
		if err != nil {
			log.Fatal(err)
		}
		fi, err := os.Stat(fpath)
		if err != nil {
			log.Fatal(err)
		}
		if fi.IsDir() {
			if err = onDirectory(fpath); err != nil {
				log.Fatal(err)
			}
		}

		//log.Println(fpath)
		return nil
	}
}
