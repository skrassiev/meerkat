package feed

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	dirStruct          = "/{aaa,aab,aac,aad,aae,aaf,aag}/{baa,bab,bac,bad,bae,baf,bag}/{caa,cab,cac,cad,cae,caf,cag}"
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

	return ret + mulArray(arr[1:])
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
	fsroot := "fsroot"
	var wg sync.WaitGroup

	defer cleanup(fsroot)

	cleanup(fsroot)

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot).Run())
	var mon1 int32
	bgFunc1 := MonitorDirectoryTree(fsroot, true, &mon1)
	ctx, cancel := context.WithCancel(context.Background())
	monitoringChannel := make(chan telega.ChattableCloser)

	wg.Add(1)
	go func() { bgFunc1(ctx, monitoringChannel); wg.Done() }()

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot+"/{aaa,aab,aac,aad,aae,aaf,aag}/{baa,bab,bac,bad,bae,baf,bag}/{caa,cab,cac,cad,cae,caf,cag}").Run())

	var mon2 int32
	bgFunc2 := MonitorDirectoryTree(fsroot, true, &mon2)

	wg.Add(1)
	go func() { bgFunc2(ctx, monitoringChannel); wg.Done() }()

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, mulArray(dirStructCount)+1, mon2)
	assert.Equal(t, mulArray(dirStructCount)+1, mon1)

	rootFS := os.DirFS(fsroot)
	totalDirs := int32(0)
	fs.WalkDir(rootFS, ".", walkFunction(fsroot, func(fpath string) error {
		//fmt.Println(fpath)
		totalDirs++
		return nil
	}))

	assert.Equal(t, mulArray(dirStructCount)+1, totalDirs)

	cleanup(fsroot)
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 0, mon2)
	assert.Equal(t, 0, mon1)

	cancel()
	wg.Wait()

}

func TestFS_MonitorDirConcurrently(t *testing.T) {
	fsroot := "fsroot"
	var wg sync.WaitGroup

	//defer cleanup(fsroot)

	//cleanup(fsroot)

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot).Run())
	var mon1 int32
	bgFunc1 := MonitorDirectoryTree(fsroot, true, &mon1)
	ctx, cancel := context.WithCancel(context.Background())
	monitoringChannel := make(chan telega.ChattableCloser)

	wg.Add(1)
	go func() { bgFunc1(ctx, monitoringChannel); wg.Done() }()
	time.Sleep(2 * time.Second)

	require.NoError(t, exec.Command("/bin/bash", "-c", "mkdir -p "+fsroot+"/{aaa,aab,aac,aad,aae,aaf,aag}/{baa,bab,bac,bad,bae,baf,bag}/{caa,cab,cac,cad,cae,caf,cag}").Run())

	time.Sleep(1500 * time.Millisecond)
	assert.Equal(t, mulArray(dirStructCount)+1, mon1)

	assert.NoError(t, fsw.Remove("fsroot/aac/bac"))

	//cleanup(fsroot)
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 0, mon1)

	cancel()
	wg.Wait()
}
