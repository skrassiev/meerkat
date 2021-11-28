package sensor

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	httpTimeout             = 30 * time.Second
	retryInterval           = 2 * time.Second
)

var (
	lastTemp       int32 = errTemp
	lastTime             = time.Now().Local().Add(-minRereshInterval)
	tickerInterval       = time.Second * 5
)

type interruptedErr struct {
	msg string
}

func (e interruptedErr) Error() string {
	return e.msg
}

// retries operation and watches for interrupt. Never return an error on success
func retryTillInterrupt(f func() error, sighandler <-chan os.Signal, runtime string) error {
	for {
		if err := f(); err != nil {
			log.Println("Telegram API failiure", err)
			select {
			case s := <-sighandler:
				if s == os.Interrupt {
					return interruptedErr{fmt.Sprintf("%s was interruped by system signal", runtime)}
				}
				return interruptedErr{fmt.Sprintf("%s was killed", runtime)}
			case <-time.After(retryInterval):
				continue
			}
		}
		break
	}
	return nil
}

// handler for a single command
type commandHandler func(cmd *tgbotapi.Message, bot *tgbotapi.BotAPI) (response tgbotapi.MessageConfig)
type unsolicitedReportFunc func() string

// ServeBotAPI is the main function
func ServeBotAPI(sighandler <-chan os.Signal, runtime string) (string, error) {

	var (
		bot *tgbotapi.BotAPI
		err error
		c   = &http.Client{Timeout: httpTimeout}
	)

	log.Println("launching sensor service")

	//	for {
	//		if bot, err = tgbotapi.NewBotAPIWithClient(os.Getenv("TELEGRAM_APITOKEN"), c); err != nil {
	//			log.Println("failed to instantiate Telegram API client", err)
	//			select {
	//			case s := <-sighandler:
	//				if s == os.Interrupt {
	//					return fmt.Sprintf("%s was interruped by system signal", runtime), nil
	//				}
	//				return fmt.Sprintf("%s was killed", runtime), nil
	//			case <-time.After(retryInterval):
	//				continue
	//			}
	//		}
	//		break
	//	}
	//

	if err = retryTillInterrupt(func() error {
		bot, err = tgbotapi.NewBotAPIWithClient(os.Getenv("TELEGRAM_APITOKEN"), c)
		return err
	}, sighandler, runtime); err != nil {
		return err.Error(), nil
	}

	strChatIDs := strings.Split(strings.TrimSpace(os.Getenv("CHAT_ID")), ",")
	if len(strChatIDs) == 0 {
		log.Fatal("CHAT_ID env var is not set or empty")
	}

	var (
		allowedChatIDs = make(map[int64]interface{})
		chatIDs        = make([]int64, 0)
	)

	for _, v := range strChatIDs {
		vv, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			log.Fatal("failed to parse chatID", v)
		}
		allowedChatIDs[vv] = struct{}{}
		chatIDs = append(chatIDs, vv)
	}

	bot.Debug = true
	// Create a new UpdateConfig struct with an offset of 0. Offsets are used
	// to make sure Telegram knows we've handled previous values and we don't
	// need them repeated.
	updateConfig := tgbotapi.NewUpdate(0)

	// Tell Telegram we should wait up to 30 seconds on each request for an
	// update. This way we can get information just as quickly as making many
	// frequent requests without having to send nearly as many.
	updateConfig.Timeout = int(httpTimeout/time.Second - 1)

	// Start polling Telegram for updates.
	updates := bot.GetUpdatesChan(updateConfig)

	// define handlers
	handlers := make(map[string]commandHandler)
	handlers["/temp"] = handleCommandlTemp
	handlers["/temp@gsnowmelt_bot"] = handleCommandlTemp

	// define periodic functions
	type reporterS struct {
		intro string
		fn    unsolicitedReportFunc
	}
	periodicUpdates := []reporterS{{"Public IP changed:", getPublicIP}}
	tm := time.NewTicker(tickerInterval)
	defer tm.Stop()

	// Let's go through each update that we're getting from Telegram.
	for {
		select {
		case s := <-sighandler:
			if s == os.Interrupt {
				return fmt.Sprintf("%s was interruped by system signal", runtime), nil
			}
			return fmt.Sprintf("%s was killed", runtime), nil
		case t := <-tm.C:
			for _, h := range periodicUpdates {
				notificationMessageWrapper(h.intro, h.fn, bot, chatIDs)
			}

		case update := <-updates:
			// Telegram can send many types of updates depending on what your Bot
			// is up to. We only want to look at messages for now, so we can
			// discard any other updates.
			if update.Message == nil {
				continue
			}

			if _, found := allowedChatIDs[update.Message.Chat.ID]; !found {
				log.Println("received", update.Message.Text[:10], "message from unknown chat")
				continue
			}

			if h, exists := handlers[update.Message.Text]; exists {
				// Okay, we're sending our message off! We don't care about the message
				// we just sent, so we'll discard it.
				log.Println("before sleep")
				time.Sleep(time.Second * 5)
				if err = retryTillInterrupt(func() error {
					_, err := bot.Send(h(update.Message, bot))
					return err
				}, sighandler, runtime); err != nil {
					return err.Error(), nil
				}
			}
		}
	}
}

func handleCommandlTemp(cmd *tgbotapi.Message, _ *tgbotapi.BotAPI) (response tgbotapi.MessageConfig) {
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

func notificationMessageWrapper(msgInfo string, messageFunc unsolicitedReportFunc, bot *tgbotapi.BotAPI, chatIDs []int64) {
	if msgText := messageFunc(); len(msgText) != 0 {
		for _, v := range chatIDs {
			msg := tgbotapi.NewMessage(v, fmt.Sprintf("%s %s", msgInfo, msgText))
			if _, err := bot.Send(msg); err != nil {
				// Note that panics are a bad way to handle errors. Telegram can
				// have service outages or network errors, you should retry sending
				// messages or more gracefully handle failures.
				panic(err)
			}
		}
	}
}
