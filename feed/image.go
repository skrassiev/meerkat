package feed

import (
	"context"
	"io"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skrassiev/meerkat/telega"
)

type chattableFileUploader struct {
	tgbotapi.PhotoConfig
	io.Closer
}

func (m chattableFileUploader) Close() error {
	return m.Closer.Close()
}

// SetChatID
func (c chattableFileUploader) SetChatID(chatID int64) {
	c.BaseChat.ChatID = chatID
}

func getRemotePictureAsBytes(ctx context.Context, url string) (body io.ReadCloser, bodyLen int64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	} else if resp != nil {
		return resp.Body, resp.ContentLength, nil
	}

	return nil, 0, nil
}

// GetPictureByURL serves an image from a remote URL.
func GetPictureByURL(fileURL string) telega.CommandHandler {
	return func(ctx context.Context, cmd *tgbotapi.Message, _ *tgbotapi.BotAPI) (response telega.ChattableCloser, _ error) {
		body, _, err := getRemotePictureAsBytes(ctx, fileURL)
		if err != nil {
			return nil, err
		}
		return chattableFileUploader{tgbotapi.NewPhoto(cmd.Chat.ID, tgbotapi.FileReader{
			Name:   "dacha.jpg",
			Reader: body,
			//Size:   len,
		}), body}, nil
	}
}
