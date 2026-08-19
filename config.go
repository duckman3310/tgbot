package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	Gray   = "\033[37m"
)

var BotInfoText string = "Бот скачивает аудио из видео на ютуб по ссылке\n\nСкачивается через yt-dlp, логи скачивания можно увидеть добавив к ссылке флаг -l\n\nКод бота открыт, посмотреть можно на https://github.com/duckman3310/tgbot"

var commandsConfig = tgbotapi.SetMyCommandsConfig{
	Commands: []tgbotapi.BotCommand{
		{
			Command:     "info",
			Description: "Информация о боте",
		},
	},
}
