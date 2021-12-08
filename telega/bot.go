package telega

import (
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handler for a single command

// ChattableCloser is a chattable message, which is closed after use
type ChattableCloser interface {
	tgbotapi.Chattable
	io.Closer
}

type ChattableText struct {
	tgbotapi.Chattable
}

func (c ChattableText) Close() error {
	return nil
}

type CommandHandler func(cmd *tgbotapi.Message, bot *tgbotapi.BotAPI) (response ChattableCloser, err error)
type TaskFunction func() string

// define periodic functions
type periodicTaskDef struct {
	interval uint32
	intro    string
	fn       TaskFunction
}

const (
	httpTimeout         = 30 * time.Second
	retryInterval       = 2 * time.Second
	minPeriodicInterval = 5 * time.Minute
)

type interruptedErr struct {
	msg string
}

func (e interruptedErr) Error() string {
	return e.msg
}

// Bot is a highger-level wrapper over tgbotpi. Allow adding service handlers and periodic functions.
type Bot struct {
	bot               *tgbotapi.BotAPI
	signalChan        <-chan os.Signal
	runtime           string
	cmdHandlers       map[string]CommandHandler
	periodicTasks     []periodicTaskDef
	periodicTaskCycle uint32
}

// Init initializes telegram bot
func (b *Bot) Init(signals <-chan os.Signal, runtime string) error {

	var err error

	log.Println("connecting bot client to API")

	c := &http.Client{Timeout: httpTimeout}
	if err = retryTillInterrupt(func() error {
		b.bot, err = tgbotapi.NewBotAPIWithClient(os.Getenv("TELEGRAM_APITOKEN"), c)
		return err
	}, signals, runtime); err != nil {
		return err
	}

	b.signalChan = signals
	b.runtime = runtime
	b.bot.Debug = true

	return nil
}

// AddHandler registers a new handler function against a command string
func (b *Bot) AddHandler(cmd string, handler CommandHandler) {
	if b.cmdHandlers == nil {
		b.cmdHandlers = make(map[string]CommandHandler)
	}
	log.Println("registered command:", cmd)
	b.cmdHandlers[cmd] = handler
}

//AddPeriodicTask registers a periodic task. Param 'interval' should be an increment of 5 mins, or it will be aligned to the next 5 mins boundary.
func (b *Bot) AddPeriodicTask(interval time.Duration, reportMessage string, fn TaskFunction) {

	taskDef := periodicTaskDef{intro: reportMessage, fn: fn}

	if interval == 0 {
		taskDef.interval = 1
	} else {
		fullIntervals := float64(interval) / float64(minPeriodicInterval)
		if fullIntervals != float64(math.Floor(fullIntervals)) {
			fullIntervals += 1
		}
		taskDef.interval = uint32(math.Floor(fullIntervals))
	}
	log.Println("added task to run every", uint32(time.Duration(taskDef.interval)*minPeriodicInterval/time.Minute), "minutes")
	b.periodicTasks = append(b.periodicTasks, taskDef)
}

// Run starts the bot till interrupted
func (b Bot) Run() (string, error) {

	// parse restrictions
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

	// Create a new UpdateConfig struct with an offset of 0. Offsets are used
	// to make sure Telegram knows we've handled previous values and we don't
	// need them repeated.
	updateConfig := tgbotapi.NewUpdate(0)

	// Tell Telegram we should wait up to 30 seconds on each request for an
	// update. This way we can get information just as quickly as making many
	// frequent requests without having to send nearly as many.
	updateConfig.Timeout = int(httpTimeout/time.Second - 1)

	// Start polling Telegram for updates.
	updates := b.bot.GetUpdatesChan(updateConfig)

	tm := time.NewTicker(minPeriodicInterval)
	defer tm.Stop()

	// Let's go through each update that we're getting from Telegram.
	for {
		select {
		case s := <-b.signalChan:
			if s == os.Interrupt {
				return fmt.Sprintf("%s was interruped by system signal", b.runtime), nil
			}
			return fmt.Sprintf("%s was killed", b.runtime), nil
		case <-tm.C:
			b.processPeriodicTasks(chatIDs)

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

			if h, exists := b.cmdHandlers[update.Message.Text]; exists {
				// Okay, we're sending our message off! We don't care about the message
				// we just sent, so we'll discard it.
				if err := retryTillInterrupt(func() error {
					outmsg, err := h(update.Message, b.bot)
					if err == nil {
						defer outmsg.Close()
						_, err = b.bot.Send(outmsg)
					}
					return err
				}, b.signalChan, b.runtime); err != nil {
					return err.Error(), nil
				}
			}
		}
	}
}

func (b *Bot) processPeriodicTasks(chatIDs []int64) {
	b.periodicTaskCycle += 1
	for _, h := range b.periodicTasks {
		if b.periodicTaskCycle/h.interval == 0 {
			notificationMessageWrapper(h.intro, h.fn, b.bot, chatIDs)
		}
	}
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

func notificationMessageWrapper(msgInfo string, messageFunc TaskFunction, bot *tgbotapi.BotAPI, chatIDs []int64) {
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
