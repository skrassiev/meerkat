package telega

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handler for a single command.

// ChattableCloser is a chattable message, which is closed after use.
type ChattableCloser interface {
	tgbotapi.Chattable
	io.Closer
	SetChatID(chatID int64)
}

// ChattableText is a simple chat message
type ChattableText struct {
	tgbotapi.MessageConfig
}

// Close is a noop function
func (c ChattableText) Close() error {
	return nil
}

// SetChatID
func (c *ChattableText) SetChatID(chatID int64) {
	c.BaseChat.ChatID = chatID
}

// ChattablePicture is a simple chat picture
type ChattablePicture struct {
	tgbotapi.PhotoConfig
}

// Close is a noop function
func (c ChattablePicture) Close() error {
	return nil
}

// SetChatID
func (c *ChattablePicture) SetChatID(chatID int64) {
	c.BaseChat.ChatID = chatID
}

// ChattableVideo is a simple chat video
type ChattableVideo struct {
	tgbotapi.VideoConfig
}

// Close is a noop function
func (c ChattableVideo) Close() error {
	return nil
}

// SetChatID
func (c *ChattableVideo) SetChatID(chatID int64) {
	c.BaseChat.ChatID = chatID
}

// ChattableDocument is a simple chat video
type ChattableDocument struct {
	tgbotapi.DocumentConfig
}

// Close is a noop function
func (c ChattableDocument) Close() error {
	return nil
}

// SetChatID
func (c *ChattableDocument) SetChatID(chatID int64) {
	c.BaseChat.ChatID = chatID
}

// CommandHandler is a function, which can handle a specific bot command
type CommandHandler func(ctx context.Context, cmd *tgbotapi.Message, bot *tgbotapi.BotAPI) (response ChattableCloser, err error)

// TaskFunction is a function, which is executed by bot periodically
type TaskFunction func(ctx context.Context) string

// BackgroundFunction runs in a background in a goroutine and sends back events as those get created
type BackgroundFunction func(ctx context.Context, events chan<- ChattableCloser)

// define periodic functions.
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
	bot                 *tgbotapi.BotAPI
	runtime             string
	cmdHandlers         map[string]CommandHandler
	periodicTasks       []periodicTaskDef
	backgroundFunctions []BackgroundFunction
	periodicTaskCycle   uint32
	ctx                 context.Context
	backgroundEvents    chan ChattableCloser
}

// Init initializes telegram bot.
func (b *Bot) Init(ctx context.Context, runtime string) error {
	var err error

	log.Println("connecting bot client to API")

	c := &http.Client{Timeout: httpTimeout}
	err = retryTillInterrupt(ctx, func(_ context.Context) error {
		b.bot, err = tgbotapi.NewBotAPIWithClient(os.Getenv("TELEGRAM_APITOKEN"), c)

		return err
	}, runtime)

	if err != nil {
		return err
	}

	b.runtime = runtime
	b.bot.Debug = true
	b.ctx = ctx
	b.backgroundEvents = make(chan ChattableCloser, 10)

	return nil
}

// AddHandler registers a new handler function against a command string.
func (b *Bot) AddHandler(cmd string, handler CommandHandler) {
	if b.cmdHandlers == nil {
		b.cmdHandlers = make(map[string]CommandHandler)
	}

	log.Println("registered command:", cmd)

	b.cmdHandlers[cmd] = handler
}

// AddPeriodicTask registers a periodic task.
// Param 'interval' should be an increment of 5 mins, or it will be aligned to the next 5 mins boundary.
func (b *Bot) AddPeriodicTask(interval time.Duration, reportMessage string, fn TaskFunction) {
	taskDef := periodicTaskDef{intro: reportMessage, fn: fn}

	if interval == 0 {
		taskDef.interval = 1
	} else {
		fullIntervals := float64(interval) / float64(minPeriodicInterval)
		if fullIntervals != math.Floor(fullIntervals) {
			fullIntervals++
		}
		taskDef.interval = uint32(math.Floor(fullIntervals))
	}

	log.Println("added task to run every", uint32(time.Duration(taskDef.interval)*minPeriodicInterval/time.Minute), "minutes")
	b.periodicTasks = append(b.periodicTasks, taskDef)
}

func (b *Bot) AddBackgroundTask(fn BackgroundFunction) {
	b.backgroundFunctions = append(b.backgroundFunctions, fn)
}

// Run starts the bot till interrupted.
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

	// launch background jobs
	var wg sync.WaitGroup
	for _, v := range b.backgroundFunctions {
		wg.Add(1)
		go func(f BackgroundFunction) {
			f(b.ctx, b.backgroundEvents)
			wg.Done()
		}(v)
	}
	defer wg.Wait()

	periodic := time.NewTicker(minPeriodicInterval)
	defer periodic.Stop()

	// Let's go through each update that we're getting from Telegram.
	for {
		select {
		case <-b.ctx.Done():
			return fmt.Sprintf("%s context cancelled", b.runtime), nil
		case <-periodic.C:
			b.processPeriodicTasks(chatIDs)

		case update := <-updates:
			// Telegram can send many types of updates depending on what your Bot
			// is up to. We only want to look at messages for now, so we can
			// discard any other updates.
			if update.Message == nil {
				continue
			}

			if _, found := allowedChatIDs[update.Message.Chat.ID]; !found {
				pos := int(math.Min(10, float64(len(update.Message.Text))))
				log.Println("received", update.Message.Text[:pos], "message from unknown chat")
				continue
			}

			if h, exists := b.cmdHandlers[strings.Split(update.Message.Text, "@")[0]]; exists {
				// Okay, we're sending our message off! We don't care about the message
				// we just sent, so we'll discard it.
				if err := retryTillInterrupt(b.ctx, func(ctx context.Context) error {
					outmsg, err := h(ctx, update.Message, b.bot)
					if err == nil {
						defer outmsg.Close()
						_, err = b.bot.Send(outmsg)
					}
					return err
				}, b.runtime); err != nil {
					return err.Error(), nil
				}
			}
		case bgEvent := <-b.backgroundEvents:

			log.Println("received BG event")

			for k, _ := range allowedChatIDs {
				bgEvent.SetChatID(k)
				log.Printf("after  %+v\n", bgEvent)
				if err := func() error {
					defer bgEvent.Close()
					return retryTillInterrupt(b.ctx, func(ctx context.Context) error {
						_, err := b.bot.Send(bgEvent)
						return err
					}, b.runtime)
				}(); err != nil {
					return err.Error(), nil
				}
			}

		}
	}
}

func (b *Bot) processPeriodicTasks(chatIDs []int64) {
	b.periodicTaskCycle++
	for _, h := range b.periodicTasks {
		if b.periodicTaskCycle/h.interval == 0 {
			notificationMessageWrapper(b.ctx, h.intro, h.fn, b.bot, chatIDs)
		}
	}
}

// retries operation and watches for interrupt. Never return an error on success.
func retryTillInterrupt(ctx context.Context, f func(ctx context.Context) error, runtime string) error {
	for {
		if err := f(ctx); err != nil {
			log.Println("Telegram API failiure", err)
			select {
			case <-ctx.Done():
				return interruptedErr{fmt.Sprintf("%s was cancelled", runtime)}
			case <-time.After(retryInterval):
				continue
			}
		}
		break
	}
	return nil
}

func notificationMessageWrapper(ctx context.Context, msgInfo string, messageFunc TaskFunction, bot *tgbotapi.BotAPI, chatIDs []int64) {
	if msgText := messageFunc(ctx); len(msgText) != 0 {
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
