package feed

import (
	"io"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skrassiev/gsnowmelt_bot/telega"
)

type chattableFileUploader struct {
	tgbotapi.PhotoConfig
	io.Closer
}

func (m chattableFileUploader) Close() error {
	return m.Closer.Close()
}

func getRemotePictureAsBytes(url string) (body io.ReadCloser, bodyLen int64, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	} else if resp != nil {
		return resp.Body, resp.ContentLength, nil
	}

	return nil, 0, nil
}

//GetPictureByURL serves an image from a remote URL
func GetPictureByURL(fileURL string) telega.CommandHandler {
	return func(cmd *tgbotapi.Message, _ *tgbotapi.BotAPI) (response telega.ChattableCloser, _ error) {
		body, len, err := getRemotePictureAsBytes(fileURL)
		if err != nil {
			return nil, err
		}
		return chattableFileUploader{tgbotapi.NewPhotoUpload(cmd.Chat.ID, tgbotapi.FileReader{
			Name:   "dacha.jpg",
			Reader: body,
			Size:   len,
		}), body}, nil
	}
}
