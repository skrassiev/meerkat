package feed

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

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

// FilterFunc accepts a filename and returns true of false if it's accepted for further processing
type FilterFunc func(fpath string) bool

var (
	// addWatch is a stock implementation of fsnotifyAdder, which does not track if fpath is already monitored.
	addWatch = func(fpath string, watcher *fsnotify.Watcher) (exists bool, err error) {
		return false, watcher.Add(fpath)
	}
	gotest = false
)

// MonitorDirectoryTree returns a function, which  watches all subdirectories for changes, starting at directory.
//
// The logic as following:
/*
   • add top-level directory to the inotify watch list;
   •  DirWalk top-level directory;
   •  for each first-level subdirectory:
    • add to the inotify watch list;
    • queue for the DirWalk;
   • skip any second-level and below subdirs: they wlil be processed as a part of walking of their parents dirs;
   • for every inotify CREATE event, check if it's a directory created, then add to the inotify list and schedule for DirWalk;
   • if a dedupe algo is used (based on tracking what's been added to inotify watch list), do not add to the list and not queue for DirWalk.
*/
// Important! First, dir should be added to inotify list, then queued for DirWalk to avoid inherent race conditions with inotify mechanism.
func MonitorDirectoryTree(directory string, filter FilterFunc) telega.BackgroundFunction {
	return monitorDirectoryTree(directory, filter, addWatch)
}

//
func monitorDirectoryTree(directory string, filter FilterFunc, fsAddWatch fsnotifyAdder) telega.BackgroundFunction {
	// we should always use a new instance of the watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	var (
		directoriesToScan = make(chan string, 100)
		modifiedFiles     = make(chan string, 100)
		fsAddWatchWrapper fsnotifyAdderWrapper
	)

	fsAddWatchWrapper = func(fpath string) (exists bool, err error) {
		return fsAddWatch(fpath, watcher)
	}

	log.Println("staring to monitor", directory)

	return func(ctx context.Context, events chan<- telega.ChattableCloser) {
		defer watcher.Close()
		done := make(chan bool)

		handleModifiedFile := func(fname string) {
			if tgEvent, err := processFile(fname); err != nil {
				log.Println("eror handling file", fname)
			} else if !gotest {
				events <- tgEvent
			}
		}

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
					if fname := onFsModification(event, fsAddWatchWrapper, directoriesToScan, filter); len(fname) != 0 {
						handleModifiedFile(fname)
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("error:", err)
				case modifiedFile := <-modifiedFiles:
					handleModifiedFile(modifiedFile)

				case <-ctx.Done():
					return
				}
			}
		}()

		if _, err = fsAddWatchWrapper(directory); err != nil {
			log.Fatalln("can't start watching possibly non-existent directory", directory)
		}
		go oneLevelDirectoryWalker(directoriesToScan, modifiedFiles, fsAddWatchWrapper, filter)
		directoriesToScan <- directory

		<-done
		close(directoriesToScan)
	}
}

func onFsModification(event fsnotify.Event, fsAdd fsnotifyAdderWrapper, walkRequests chan<- string, filter FilterFunc) (newFileName string) {
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
		} else { //if (event.Op & fsnotify.Write) != 0 {
			// it's a newly created file
			//log.Println("modified or created file:", event.Name)
			if filter(event.Name) {
				log.Println(event.Name, "accepted")
				return event.Name
			}
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
	log.Println("process file", fname)
	return &telega.ChattablePicture{
		PhotoConfig: tgbotapi.NewPhotoUpload(0, fname),
	}, nil
}

func oneLevelDirectoryWalker(fpaths <-chan string, modifiedFiles chan<- string, fsAdd fsnotifyAdderWrapper, filter FilterFunc) {
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
				if d.IsDir() {
					if exists, err := fsAdd(fullPath); err != nil {
						log.Println("failed to add directory", fullPath, "watch:", err)
						return err
					} else if !exists && strings.Count(fpath, "/") == 0 {
						// only requesting to walk a first-level directory
						directoriesToWalk = append(directoriesToWalk, fullPath)
					}
					// never decend but to the frist level (see above for ".")
					return fs.SkipDir
				}
				if strings.Count(fpath, "/") == 0 && filter(fpath) {
					// it's ok if it blocks. That might happen in two cases:
					// when there are lots of files in the dir
					// or telegram bot is not connected to the server.
					// In both cases, it's ok to slow down walk function.
					modifiedFiles <- fullPath
				}
				return nil
			})

		}

	}
	log.Println("exiting WALKER")
}

// FilenameFilter accepts an array of regexp string to match a file name against.
// The regexps in array must be valid.
// Returned is a filter function.
func FilenameFilter(regexpPatterns []string) FilterFunc {
	regexps := make([]*regexp.Regexp, len(regexpPatterns))

	for i := range regexpPatterns {
		regexps[i] = regexp.MustCompile(regexpPatterns[i])
	}

	return func(fname string) bool {
		for _, v := range regexps {
			if v.MatchString(fname) {
				return true
			}
		}
		return false
	}
}

// NewFileFilterChain returns only if a fle is created after the launch
func NewfileFilterChain(filter FilterFunc) FilterFunc {
	started := time.Now()
	return func(fpath string) bool {
		if filter(fpath) {
			if stat, err := os.Stat(fpath); err == nil {
				if stat.ModTime().After(started) {
					return true
				}
			}
		}
		return false
	}
}
