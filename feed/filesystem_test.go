package feed

import (
	"context"
	"io/fs"
	"log"
	"mime"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/skrassiev/gsnowmelt_bot/telega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fsnotifyNoEntError = "can't remove non-existent inotify watch for"
	dirStruct          = "/{aaa,aab,aac,aad,aae,aaf,aag}/{baa,bab,bac,bad,bae,baf,bag}/{caa,cab,cac,cad,cae,caf,cag}/{foo,bar,baz}i/{alpha,beta,gamma,theta}"
)

var (
	dirStructCount = func() []int32 {
		ds := strings.Split(dirStruct, "/")
		if len(ds) > 0 && len(ds[0]) == 0 {
			ds = ds[1:]
		}

		var ret []int32
		for i := range ds {
			ret = append(ret, int32(len(strings.Split(ds[i], ","))))
		}
		log.Printf("%+v\n", ret)
		return ret
	}()
)

func mulArray(arr []int32) int32 {
	if len(arr) == 0 {
		return 0
	}

	var ret int32 = 1

	for i := range arr {
		ret *= arr[i]
	}

	return ret + mulArray(arr[:len(arr)-1])
}

func cleanup(fpath string) {
	exec.Command("/bin/bash", "-c", "rm -rf "+fpath).Run()
}

func TestFS_fsnotify(t *testing.T) {

	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()

	pwd := "./"
	if fs.ValidPath("filesystem_test.go") {
		pwd = "../"
	}

	assert.True(t, true)
	assert.NoError(t, watcher.Add(pwd+"feed"))
	assert.NoError(t, watcher.Add(pwd+"feed"))
	assert.EqualValues(t, syscall.ENOENT, watcher.Add(pwd+pwd+"feed"))

	err = watcher.Remove(pwd + pwd + "feed")
	require.Error(t, err)
	assert.Contains(t, err.Error(), fsnotifyNoEntError)

	assert.NoError(t, watcher.Remove(pwd+"feed"))

	err = watcher.Remove(pwd + "feed")
	require.Error(t, err)
}

func TestFS_MonitorDirRecursively(t *testing.T) {

	var (
		fsMap            [2]sync.Map
		monitoredCounter [2]int32
		fsAdd1           = func(fpath string, watcher *fsnotify.Watcher) (exists bool, err error) {
			if _, loaded := fsMap[0].LoadOrStore(fpath, struct{}{}); !loaded {
				atomic.AddInt32(&monitoredCounter[0], 1)
				return false, watcher.Add(fpath)
			}
			return true, nil
		}
		fsAdd2 = func(fpath string, watcher *fsnotify.Watcher) (exists bool, err error) {
			if _, loaded := fsMap[1].LoadOrStore(fpath, struct{}{}); !loaded {
				atomic.AddInt32(&monitoredCounter[1], 1)
				return false, watcher.Add(fpath)
			}
			return true, nil
		}
	)

	fsroot := "fsroot"
	var wg sync.WaitGroup

	defer cleanup(fsroot)

	cleanup(fsroot)

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot).Run())
	bgFunc1 := monitorDirectoryTree(fsroot, FilenameFilter([]string{`\.jpg$/i`}), fsAdd1)
	ctx, cancel := context.WithCancel(context.Background())
	monitoringChannel := make(chan telega.ChattableCloser)

	wg.Add(1)
	go func() { bgFunc1(ctx, monitoringChannel); wg.Done() }()

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot+dirStruct).Run())

	bgFunc2 := monitorDirectoryTree(fsroot, FilenameFilter([]string{`\.jpg$/i`}), fsAdd2)

	wg.Add(1)
	go func() { bgFunc2(ctx, monitoringChannel); wg.Done() }()

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, mulArray(dirStructCount)+1, monitoredCounter[0])
	assert.Equal(t, mulArray(dirStructCount)+1, monitoredCounter[1])

	cleanup(fsroot)

	cancel()
	wg.Wait()

}

func TestFS_MonitorDirConcurrently(t *testing.T) {

	var (
		fsMap            sync.Map
		monitoredCounter int32
		fsw              *fsnotify.Watcher
		fsAdd            = func(fpath string, watcher *fsnotify.Watcher) (exists bool, err error) {
			fsw = watcher
			if _, loaded := fsMap.LoadOrStore(fpath, struct{}{}); !loaded {
				atomic.AddInt32(&monitoredCounter, 1)
				return false, watcher.Add(fpath)
			}
			return true, nil
		}
	)

	fsroot := "fsroot"
	var wg sync.WaitGroup

	defer cleanup(fsroot)

	cleanup(fsroot)

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot).Run())
	bgFunc1 := monitorDirectoryTree(fsroot, FilenameFilter([]string{`\.jpg$/i`}), fsAdd)
	ctx, cancel := context.WithCancel(context.Background())
	monitoringChannel := make(chan telega.ChattableCloser)

	wg.Add(1)
	go func() { bgFunc1(ctx, monitoringChannel); wg.Done() }()

	err := exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot+dirStruct).Run()
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, mulArray(dirStructCount)+1, monitoredCounter)

	assert.NoError(t, fsw.Remove("fsroot/aac/bac"))

	cleanup(fsroot)
	time.Sleep(100 * time.Millisecond)

	cancel()
	wg.Wait()
}

func TestFS_MonitorDirFiles(t *testing.T) {

	gotest = true
	defer func() { gotest = false }()

	var (
		fsMap            sync.Map
		monitoredCounter int32
		fsw              *fsnotify.Watcher
		fsAdd            = func(fpath string, watcher *fsnotify.Watcher) (exists bool, err error) {
			fsw = watcher
			if _, loaded := fsMap.LoadOrStore(fpath, struct{}{}); !loaded {
				atomic.AddInt32(&monitoredCounter, 1)
				return false, watcher.Add(fpath)
			}
			return true, nil
		}
	)

	fsroot := "fsroot"
	var wg sync.WaitGroup

	//defer cleanup(fsroot)

	cleanup(fsroot)

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot).Run())
	bgFunc1 := monitorDirectoryTree(fsroot, FilenameFilter([]string{`(?i)\.jpg$`}), fsAdd)
	ctx, cancel := context.WithCancel(context.Background())
	monitoringChannel := make(chan telega.ChattableCloser)

	wg.Add(1)
	go func() { bgFunc1(ctx, monitoringChannel); wg.Done() }()

	err := exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot+dirStruct).Run()
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, mulArray(dirStructCount)+1, monitoredCounter)

	testFiles := []struct {
		level     int
		filenames []string
	}{
		{
			level:     2,
			filenames: []string{"foo.txt", "bar.jpg", "baz.JPG", "foobar.png"},
		},
		{
			level:     3,
			filenames: []string{"foo.txt", "bar.jPg", "baz.JPG", "foobar.png"},
		}, {
			level:     3,
			filenames: []string{"foo.jpg", "bar1.jpg", "baz.JPG", "foobar.png"},
		}, {
			level:     2,
			filenames: []string{"foo.txt", "baz.JPG", "foobar.png"},
		},
	}
	times := len(testFiles) - 1

	rootFS := os.DirFS(fsroot)
	fs.WalkDir(rootFS, ".", func(fpath string, d fs.DirEntry, err error) error {
		if fpath == "." {
			// enumerate top-level directory content
			return nil
		}

		if times >= 0 {
			if strings.Count(fpath, "/") == testFiles[times].level {
				for _, v := range testFiles[times].filenames {

					if f, err := os.Create(path.Join(fsroot, fpath, v)); err == nil {
						f.Close()
					}
				}
				times--
			}
		} else {
			return fs.SkipDir
		}
		return nil
	})

	assert.NoError(t, fsw.Remove("fsroot/aac/bac"))

	cleanup(fsroot)
	time.Sleep(100 * time.Millisecond)

	cancel()
	wg.Wait()
}

func TestMIME_Std(t *testing.T) {
	assert.Equal(t, "video", strings.Split(mime.TypeByExtension(path.Ext("foo/bar/baz/add.mP4")), "/")[0])
	assert.Equal(t, "image", strings.Split(mime.TypeByExtension(path.Ext("baz/bar/foo/pic.jpG")), "/")[0])
}
