package feed

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/fsnotify/fsnotify"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

// fsnotifyAdder adds fpath to inotify watch list. Returns error if fails to add and exists == false.
// If path is already monitored, exists returns true and no attempt to add is made, so err is always nil.
// Otherwise, if exists == false and err == nil, adding was successul.
// No all implementations of fsnotifyAdder function can actually track if fpath is already monitored.
type fsnotifyAdder func(fpath string, watcher *fsnotify.Watcher) (exists bool, err error)

type fsnotifyAdderWrapper func(fpath string) (exists bool, err error)

var (
	// addWatch is a stock implementation of fsnotifyAdder, which does not track if fpath is already monitored.
	addWatch = func(fpath string, watcher *fsnotify.Watcher) (exists bool, err error) {
		return false, watcher.Add(fpath)
	}
)

// MonitorDirectoryTree watches all subdirectories for changes, starting at directory.
// The logic as following:
// - add top-level directory to the inotify watch list;
// - DirWalk top-level directory;
// - for each first-level subdirectory:
// -- add to the inotify watch list;
// -- queue for the DirWalk;
// - skip any second-level and below subdirs: they wlil be processed as a part of walking of their parents dirs;
// - for every inotify CREATE event, check if it's a directory created, then add to the inotify list and schedule for DirWalk;
// - if a dedupe algo is used (based on tracking what's been added to inotify watch list), do not add to the list and not queue for DirWalk.
// Important! First, dir should be added to inotify list, then queued for DirWalk to avoid inherent race conditions with inotify mechanism.
func MonitorDirectoryTree(directory string) telega.BackgroundFunction {
	return monitorDirectoryTree(directory, addWatch)
}

//
func monitorDirectoryTree(directory string, fsAddWatch fsnotifyAdder) telega.BackgroundFunction {
	// we should always use a new instance of the watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	var (
		directoriesToScan = make(chan string, 100)
		fsAddWatchWrapper fsnotifyAdderWrapper
	)

	fsAddWatchWrapper = func(fpath string) (exists bool, err error) {
		return fsAddWatch(fpath, watcher)
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
					//log.Println("event:", event)
					if fname := onFsModification(event, fsAddWatchWrapper, directoriesToScan); len(fname) != 0 {
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

		if _, err = fsAddWatchWrapper(directory); err != nil {
			log.Fatalln("can't start watching possibly non-existent directory", directory)
		}
		go oneLevelDirectoryWalker(directoriesToScan, fsAddWatchWrapper)
		directoriesToScan <- directory

		<-done
		close(directoriesToScan)
	}
}

func onFsModification(event fsnotify.Event, fsAdd fsnotifyAdderWrapper, walkRequests chan<- string) (newFileName string) {
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
			if exists, err := fsAdd(event.Name); err != nil {
				log.Println("failed to add directory", event.Name, "watch:", err)
				return
			} else if !exists {
				walkRequests <- event.Name
			}
		} else if (event.Op & fsnotify.Write) != 0 {
			// it's a newly created file
			log.Println("modified or created file:", event.Name)
			return event.Name
		}
	} else if (event.Op & fsnotify.Remove) != 0 {
		// We don't know if it was a dir or file.
		// On top of that, it has alredy been removed by fsnotify.
		// Moreover, fsnotify package maps both IN_DELETE (subdirectory is removed from the parent)
		// and IN_DELETE_SELF (the actual subdirectory removal) to the same Op Delete,
		// so there will be two Delete events for the same event.Name directory.
	}

	return
}

func processFile(fname string) (telega.ChattableCloser, error) {
	return telega.ChattableText{
		Chattable: tgbotapi.NewMessage(0, fmt.Sprintf("file %s added", fname)),
	}, nil
}

func oneLevelDirectoryWalker(fpaths <-chan string, fsAdd fsnotifyAdderWrapper) {
	for fp := range fpaths {
		directoriesToWalk := []string{fp}

		for len(directoriesToWalk) > 0 {
			nextdir := directoriesToWalk[len(directoriesToWalk)-1]
			directoriesToWalk = directoriesToWalk[:len(directoriesToWalk)-1]

			rootFS := os.DirFS(nextdir)
			fs.WalkDir(rootFS, ".", func(fpath string, d fs.DirEntry, err error) error {
				if fpath == "." {
					// enumerate top-level directory content
					return nil
				}
				fullPath := path.Join(nextdir, fpath)
				if exists, err := fsAdd(fullPath); err != nil {
					log.Println("failed to add directory", fullPath, "watch:", err)
					return err
				} else if !exists && strings.Count(fpath, "/") == 0 {
					// only requesting to walk a first-level directory
					directoriesToWalk = append(directoriesToWalk, fullPath)
				}
				// never decend but to the frist level (see above for ".")
				return fs.SkipDir
			})

		}

	}
	log.Println("exiting WALKER")
}
