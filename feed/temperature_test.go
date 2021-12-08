package feed

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testDataDir = func() string {
	_, err := os.Stat("testdata")
	if os.IsNotExist(err) {
		return "../testdata"
	}
	return "testdata"
}()

func Test001_tempParseBad(t *testing.T) {
	for _, v := range []string{"23452525", "", " ", "  ", "234525 25", "1 2 3 ", "23452525\nadasd", "23452525\n", "3 4  5 c=asdf", "1 2 3 a t=\nsdf sf 3 c=34", "1 2 3 c=32424\n 3 4 5 t=", "3 44 55 6 c=\nt=", "2 3 4 5 v=\nt="} {

		assert.Error(t, func() error {
			_, err := scanTemperatureReading(strings.NewReader(v))
			return err
		}())
	}
}

func Test002_tempParseGood(t *testing.T) {
	for _, v := range []string{"23452525\n3 t=234", "1 2 3 \n4 6   t=346", "1 2 3\ndf t=346", "1 2 \n3 4 5 t=-123144 "} {
		assert.NoError(t, func() error {
			_, err := scanTemperatureReading(strings.NewReader(v))
			return err
		}())
	}
}

func Test003_tempParseFile(t *testing.T) {
	v, err := getTemperatureReading(testDataDir + sensorDevicePath)
	assert.NoError(t, err)
	assert.Equal(t, int32(29812), v)

}

func Test004_tempParseFilePersistent(t *testing.T) {
	v, err := getTemperatureReadingWithRetries(testDataDir+sensorDevicePath, 11)
	assert.NoError(t, err)
	assert.Equal(t, int32(29812), v)

	v, err = getTemperatureReadingWithRetries(testDataDir+sensorDevicePath, 1000)
	assert.NoError(t, err)
	assert.Equal(t, int32(29812), v)

	lastTime = time.Now().Add(-minRereshInterval)
	v, err = getTemperatureReadingWithRetries(testDataDir+sensorDevicePath, -1)
	assert.NoError(t, err)
	assert.Equal(t, int32(29812), v)

	lastTime = time.Now().Add(-minRereshInterval)
	lastTemp = errTemp
	v, err = getTemperatureReadingWithRetries(testDataDir+sensorDevicePath, 0)
	assert.NoError(t, err)
	assert.Equal(t, int32(29812), v)
}
