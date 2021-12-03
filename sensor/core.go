package sensor

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"io"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	sensorDevicePath        = "/sys/bus/w1/devices/28-3c01d607cfc6/w1_slave"
	errTemp           int32 = -1000
	maxRetries              = 10
	minRereshInterval       = 5 * time.Second
)

var (
	lastTemp       int32 = errTemp
	lastTime             = time.Now().Local().Add(-minRereshInterval)
	tickerInterval       = time.Second * 60 * 5
)

// HandlerCommandTemp reads temp from a sensor and reponds in a telegram message
func HandleCommandlTemp(cmd *tgbotapi.Message, _ *tgbotapi.BotAPI) (response tgbotapi.MessageConfig) {
	v, _ := getTemperatureReadingWithRetries(sensorDevicePath, 10)
	// Now that we know we've gotten a new message, we can construct a
	// reply! We'll take the Chat ID and Text from the incoming message
	// and use it to create a new message.
	response = tgbotapi.NewMessage(cmd.Chat.ID, fmt.Sprintf("%v ℃ 🌡 on %v", float32(v)/1000.0, lastTime.Format("Jan 2 15:04:05")))
	// We'll also say that this message is a reply to the previous message.
	// For any other specifications than Chat ID or Text, you'll need to
	// set fields on the `MessageConfig`.
	// msg.ReplyToMessageID = update.Message.MessageID
	return
}

func scanTemperatureReading(reader io.Reader) (int32, error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		ss := strings.Split(strings.TrimSpace(scanner.Text()), " ")
		if len(ss) > 1 {
			ts := ss[len(ss)-1]
			if strings.HasPrefix(ts, "t=") {
				st := strings.Split(ts, "=")
				if len(st) == 2 {
					ret, err := strconv.Atoi(st[1])
					if err != nil {
						log.Println(err, st[1])
						return errTemp, err
					}
					return int32(ret), nil
				}
				log.Println("could not parse", ts)
				return errTemp, fmt.Errorf("could not parse %v", ts)

			}
		}
	}

	log.Println("no temp pattern found")

	return errTemp, errors.New("no temp pattern found")
}

func getTemperatureReading(fpath string) (int32, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return errTemp, err
	}

	defer func() { _ = f.Close() }()
	return scanTemperatureReading(f)
}

func getTemperatureReadingWithRetries(fpath string, retries int) (temperature int32, err error) {
	// do not allow more frequent polls
	if time.Now().Sub(lastTime) < minRereshInterval {
		return atomic.LoadInt32(&lastTemp), nil
	}

	if retries > maxRetries {
		retries = maxRetries
	} else if retries <= 0 {
		retries = 1
	}

	for ; retries >= 0; retries-- {
		if temperature, err = getTemperatureReading(fpath); err == nil {
			// sometimes the temperature is just not refreshed by a sensor. Retry few times
			if lt := atomic.LoadInt32(&lastTemp); lt != temperature {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err == nil {
		atomic.StoreInt32(&lastTemp, temperature)
		lastTime = time.Now() // ignore concurrency issues
	} else {
		temperature = atomic.LoadInt32(&lastTemp)
	}

	return
}
