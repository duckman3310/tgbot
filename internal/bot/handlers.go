package bot

import (
	"context"
	"fmt"
	"tgbot/internal/downloader"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (bot *Bot) handleRequest(chatID int64, messageID int, messageText string) {

	logBot(fmt.Sprintf("Получено: %s", messageText))

	request := downloader.NewDownloadRequest(messageText)
	fmt.Println(request.URL)

	infoCtx, infoCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer infoCancel()

	audioInfo, err := downloader.DownloadAudioInfo(infoCtx, request.URL)
	if err != nil {
		logErr("Cant download audioInfo", err)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Не удалось скачать информацию об аудио, ошибка: %v", err))
		if _, err := bot.API.Send(msg); err != nil {
			logErr("Cant send audioInfo download error", err)
		}
		return
	}

	fmt.Println(audioInfo)
	audioCtx, audioCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer audioCancel()

	audioData, err := downloader.DownloadAudio(audioCtx, request)
	if err != nil {
		logErr("Cant download audio", err)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Не удалось скачать аудио, ошибка: %v", err))
		if _, err := bot.API.Send(msg); err != nil {
			logBot(fmt.Sprintf("Cant send audio download error: %v", err))
		}
		return
	}

	audio := tgbotapi.NewAudio(chatID, tgbotapi.FileBytes{
		Name:  fmt.Sprintf("%s.%s", audioInfo.Title, audioInfo.Ext),
		Bytes: audioData,
	})

	audio.Title = audioInfo.Title
	audio.Performer = audioInfo.Uploader

	if _, err := bot.API.Send(audio); err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Не удалось отправить аудио, ошибка: %v", err))
		if _, err := bot.API.Send(msg); err != nil {
			logBot(fmt.Sprintf("Cant send audio send error: %v", err))
		}
		return
	}
}
