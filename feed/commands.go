package feed

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skrassiev/meerkat/telega"
)

// PingCommand reads temp from a sensor and reponds in a telegram message.
func PingCommand(ctx context.Context, cmd *tgbotapi.Message, _ *tgbotapi.BotAPI) (response telega.ChattableCloser, _ error) {
	r := tgbotapi.NewMessage(cmd.Chat.ID, "pong")
	return &telega.ChattableText{MessageConfig: r}, nil
}
