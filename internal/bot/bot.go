package bot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"tgbot/internal/downloader"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func BotInit() {

	// загружаем конфиг
	botcfg := NewBotCfg()
	if err := botcfg.Load(); err != nil {
		log.Printf(Red+"Не удалось загрузить конфигурацию"+Reset+": %v", err)
		return
	}

	// инициализируем бота
	bot, err := tgbotapi.NewBotAPI(botcfg.Token)
	if err != nil {
		log.Printf(Red+"Не удалось инициализировать бота"+Reset+": %v", err)
		return
	}

	// отправляем команды
	if _, err := bot.Request(commandsConfig); err != nil {
		log.Printf(Red+"Не удалось отправить команды"+Reset+": %v", err)
	}

	// настройка
	bot.Debug = false
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	log.Printf(Green + "Бот запущен" + Reset)

	for update := range updates {

		// если ничего нету скип
		if update.Message == nil || update.Message.Text == "" {
			continue
		}

		messageText := update.Message.Text
		chatID := update.Message.Chat.ID
		messageID := update.Message.MessageID

		fmt.Printf("Chat ID: %d\n", chatID)

		// если команда
		if update.Message.IsCommand() {
			switch update.Message.Text {
			case "/info":
				if botcfg.InfoText != "" {
					if _, err = bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, botcfg.InfoText)); err != nil {
						log.Printf(Red+"Не удалось отправить ответ на команду /info"+Reset+": %v", err)
					}
				}
			}
			continue
		}

		showLog := false
		var link string

		// cобираем параметры из всех аргументов
		for _, arg := range strings.Fields(messageText) {
			if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				link = arg
			} else if arg == "-l" {
				showLog = true
			}
		}

		// если чет есть то скачиваем
		if link != "" {
			go downloadAndSend(bot, chatID, messageID, link, showLog, botcfg.SongQuality)
		}
	}
}

func downloadAndSend(bot *tgbotapi.BotAPI, chatID int64, userMsgID int, url string, showlog bool, quality string) {
	log.Printf("Получена ссылка: %s", url)

	// ставим таймер
	startTime := time.Now()

	// удаляем сообщение с ссылкой от пользователя
	if _, err := bot.Request(tgbotapi.NewDeleteMessage(chatID, userMsgID)); err != nil {
		log.Printf(Red+"Не удалось отправить запрос на удаление отправленой ссылки"+Reset+": %v", err)
	}

	// создаем и отправляем статус месседж
	statusMsg, err := bot.Send(tgbotapi.NewMessage(chatID, "Скачивается.."))
	if err != nil {
		log.Printf(Red+"Не удалось отправить statusMsg, %s"+Reset, err)
	}

	// Уникальный ID текущего скачивания (чтобы параллельные потоки не путали файлы)
	reqID := time.Now().UnixNano()
	outputTemplate := fmt.Sprintf("track_%d_%d_%%(title)s.%%(ext)s", chatID, reqID)

	log.Printf("попытка скачивания через yt-dlp")

	// скачивание через yt-dlp
	downloadlog, err := downloader.YtdlpDownload(outputTemplate, url, quality)

	if err != nil {
		log.Printf(Red+"Не удалось скачать файл, %s"+Reset, err)
		if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, "Не удалось скачать:\n\n"+url)); err != nil {
			log.Printf(Red+"Не удалось отправить уведомление о ошибке скачивания"+Reset+": %v", err)
		}
		return
	}

	// 1. Ищем аудиофайл .m4a строго по уникальному reqID этого запроса
	audioFiles, err := filepath.Glob(fmt.Sprintf("track_%d_%d_*.m4a", chatID, reqID))
	if err != nil || len(audioFiles) == 0 {
		log.Printf(Red + "Скачанный m4a файл не найден" + Reset)
		if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, "Файл затерялся лол")); err != nil {
			log.Printf(Red+"Не удалось отправить уведомление о потеряном файле"+Reset+": %v", err)
		}
		return
	}

	filePath := audioFiles[0]

	// Гарантируем удаление .m4a файла с сервера при завершении функции
	defer func() {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			log.Printf(Red+"Не удалось удалить файл с сервера"+Reset+": %v", err)
		}
	}()

	actualExt := filepath.Ext(filePath)

	var fileSizeMB float64
	if fileInfo, err := os.Stat(filePath); err == nil {
		fileSizeMB = float64(fileInfo.Size()) / 1024 / 1024
	}

	// Собираем имя файла без уникального префикса и расширения
	cleanName := filepath.Base(filePath)
	cleanName = strings.TrimPrefix(cleanName, fmt.Sprintf("track_%d_%d_", chatID, reqID))
	cleanName = strings.TrimSuffix(cleanName, actualExt)

	log.Printf(Green+"Скачан файл: "+Reset+"%s%s %.2f MB", cleanName, actualExt, fileSizeMB)

	// Меняем статус в Телеграме
	if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, fmt.Sprintf("%s Отправляется..", cleanName))); err != nil {
		log.Printf(Red+"Не удалось отправить уведомление о скачанном файле"+Reset+": %v", err)
	}

	// 2. Ищем обложку СТРОГО по точному имени конкретно этого .m4a файла
	var thumbPath string
	possibleThumb := strings.TrimSuffix(filePath, actualExt) + ".jpg"
	if _, err := os.Stat(possibleThumb); err == nil {
		thumbPath = possibleThumb
		defer func(p string) {
			_ = os.Remove(p)
		}(thumbPath)
	}

	// Создаем объект аудио
	audioFile := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(filePath))
	audioFile.Title = cleanName
	audioFile.Performer = ""

	// Прикрепляем обложку
	if thumbPath != "" {
		audioFile.Thumb = tgbotapi.FilePath(thumbPath)
	}

	// Отправляем файл
	if _, err := bot.Send(audioFile); err != nil {
		log.Printf(Red+"Телеграм отклонил отправку файла"+Reset+": %v", err)

		if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, "Отправить не удалось\n\n(сообщение можно удалить)")); err != nil {
			log.Printf(Red+"Не удалось отправить уведомление об отклонении отправки файла"+Reset+": %v", err)
		}
		return
	}

	log.Printf("Файл успешно отправлен, удаление с сервера")

	// Удаляем временный статус
	if _, err = bot.Request(tgbotapi.NewDeleteMessage(chatID, statusMsg.MessageID)); err != nil {
		log.Printf(Red+"Не удалось удалить статус месседж"+Reset+": %v", err)
	}

	// отправка лога скачивания
	if showlog == true {
		if downloadlog == nil {
			if _, err = bot.Send(tgbotapi.NewMessage(chatID, "Лог пустой, yt-dlp скорее всего не сmogg запустится")); err != nil {
				log.Printf(Red+"Не удалось отправить уведомление о пустом логе"+Reset+": %v", err)
			}
		}

		downloadTime := time.Since(startTime).Seconds()
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("скачивание и отправка заняло %.1fc,\nкачество: %s\n```\n%s\n```", downloadTime, quality, strings.Join(downloadlog, "\n")))
		msg.ParseMode = tgbotapi.ModeMarkdown

		if _, err = bot.Send(msg); err != nil {
			log.Printf(Red+"Не удалось отправить лог скачивания"+Reset+": %v", err)
		}
	}

	endTime := time.Since(startTime).Seconds()
	log.Printf(Green+"Завершено, %.1fc"+Reset, endTime)
}
