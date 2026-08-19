package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	log.Printf("Инициализация бота")

	// создаем бота
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Printf(Red+"Инициализация не удалась"+Reset+": %v", err)
		return
	}

	log.Printf("Отправка команд")

	// отправляем команды
	if _, err := bot.Request(commandsConfig); err != nil {
		log.Printf(Red+"Не удалось отправить команды"+Reset+": %v", err)
	}

	// настройка
	bot.Debug = false
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	log.Printf(Green + "Бот успешно запущен" + Reset)

	// цикл обработки входящих сообщений
	for update := range updates {

		// если ничего нету скип
		if update.Message == nil || update.Message.Text == "" {
			continue
		}

		// если пришло чтото дельное получаем данные из месседжа
		msgText := update.Message.Text
		chatID := update.Message.Chat.ID
		messageID := update.Message.MessageID

		// если команда
		if update.Message.IsCommand() {
			switch update.Message.Text {
			case "/info":
				// отправляем инфо
				if _, err = bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, BotInfoText)); err != nil {
					log.Printf(Red+"Не удалось отправить ответ на команду (info)"+Reset+": %v", err)
				}
			}
			continue
		}

		showLog := false
		var link string

		// cобираем параметры из всех аргументов
		for _, arg := range strings.Fields(msgText) {
			if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				link = arg
			} else if arg == "-l" {
				showLog = true
			}
		}

		// если чет есть то скачиваем
		if link != "" {
			go downloadAndSend(bot, chatID, messageID, link, showLog)
		}
	}
}

func downloadAndSend(bot *tgbotapi.BotAPI, chatID int64, userMsgID int, url string, showlog bool) {
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

	// Шаблон имени файла
	outputTemplate := fmt.Sprintf("track_%d_%%(title)s.%%(ext)s", chatID)

	log.Printf("попытка скачивания через yt-dlp")

	// скачивание через yt-dlp, обработка ошибок и отправка лога
	downloadlog, err := RunDownloadCmd(outputTemplate, url)

	if err != nil {
		log.Printf(Red+"Не удалось скачать файл, %s"+Reset, err)
		if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, "Не удалось скачать:\n\n"+url)); err != nil {
			log.Printf(Red+"Не удалось отправить уведомление о ошибке скачивания"+Reset+": %v", err)
		}
		return
	}

	if showlog == true {
		if downloadlog == nil {
			if _, err = bot.Send(tgbotapi.NewMessage(chatID, "Лог пустой, yt-dlp скорее всего не сmogg запустится")); err != nil {
				log.Printf(Red+"Не удалось отправить уведомление о пустом логе"+Reset+": %v", err)
			}
		}

		downloadTime := time.Since(startTime).Seconds() // время скачиванияы
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("скачано за %.1fc\n```\n%s\n```", downloadTime, strings.Join(downloadlog, "\n")))
		msg.ParseMode = tgbotapi.ModeMarkdown // включаем разметку Markdown

		// отправляем
		if _, err = bot.Send(msg); err != nil {
			log.Printf(Red+"Не удалось отправить лог скачивания"+Reset+": %v", err)
		}
	}

	// поиск скаченого файла и обработка ошибок
	files, _ := filepath.Glob(fmt.Sprintf("track_%d_*", chatID))
	if len(files) == 0 {
		log.Printf(Red + "Скачанный файл не найден" + Reset)
		if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, "Файл затерялся лол")); err != nil {
			log.Printf(Red+"Не удалось отправить уведомнение о потеряном файле"+Reset+": %v", err)
		}
		return
	}

	// берем из масива файл, ожидаемых файлов один,
	// поэтому обращяемся по нулевому индексу
	filePath := files[0]

	// получаем формат файла
	actualExt := filepath.Ext(filePath)

	// получаем общие данные о файле, из которых берем размер
	fileInfo, _ := os.Stat(filePath)
	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024

	// собираем красивое имя для файла
	cleanName := filepath.Base(filePath)
	cleanName = strings.TrimPrefix(cleanName, fmt.Sprintf("track_%d_", chatID))
	cleanName = strings.TrimSuffix(cleanName, actualExt)

	log.Printf(Green+"Скачан файл: "+Reset+"%s%s %.2f MB", cleanName, actualExt, fileSizeMB)

	// Меняем статус в телеге
	if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, fmt.Sprintf("%s Отправляется..", cleanName))); err != nil {
		log.Printf(Red+"Не удалось отправить уведомление о скачанном файле"+Reset+": %v", err)
	}

	// Создаем объект аудио
	audioFile := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(filePath))

	// теги
	audioFile.Title = cleanName
	audioFile.Performer = ""

	// отправляем файл
	if _, err := bot.Send(audioFile); err != nil {
		log.Printf(Red+"Телеграм отклонил отправку файла"+Reset+": %v", err)

		if _, err = bot.Send(tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, "Отправить не удалось\n\n(сообщение можно удалить)")); err != nil {
			log.Printf(Red+"Не удалось отправить уведомление об отклонении отправки файла"+Reset+": %v", err)
		}
	}

	log.Printf("Файл успешно отправлен, удаление с сервева")

	// удаляем файл с сервера
	if err = os.Remove(filePath); err != nil {
		log.Printf(Red+"Не удалось удалить файл с сервера"+Reset+": %v", err)

	}

	// Удаляем временный статус
	if _, err = bot.Request(tgbotapi.NewDeleteMessage(chatID, statusMsg.MessageID)); err != nil {
		log.Printf(Red+"Не удалось удалить статус месседж"+Reset+": %v", err)

	}

	// считаем итоговое время и завершаем функцию
	endTime := time.Since(startTime).Seconds()
	log.Printf(Green+"Завершено, %.1fc"+Reset, endTime)
}

// скачивает аудио дорожку в формате m4a из видео на ютуб по ссылке
func RunDownloadCmd(outputTemplate, url string) ([]string, error) {

	// создаем пустой лог
	downloadLog := []string{}

	// создаем комманду для скачивания
	cmd := exec.Command(
		"yt-dlp",
		"--newline",
		"--js-runtimes", "node",
		"-f", "ba",
		"-x",
		"--audio-format", "m4a",
		"-o", outputTemplate,
		url,
	)

	pipeReader, pipeWriter := io.Pipe()

	// подключаем врайтер к выходам запускаемой проги
	cmd.Stdout = pipeWriter
	cmd.Stderr = pipeWriter

	// запускаем cmd
	if err := cmd.Start(); err != nil {
		_ = pipeWriter.Close()
		return nil, err
	}

	// дожидаемся закрытия программыв в отдельной горутине
	go func() {
		err := cmd.Wait()
		_ = pipeWriter.CloseWithError(err)
	}()

	// Создаем буфер для вывода yt-dlp из трубы, сканер проще говоря
	scanner := bufio.NewScanner(pipeReader)

	// сканируем все что выдает yt-dlp
	for scanner.Scan() {
		line := scanner.Text()

		downloadLog = append(downloadLog, line)

		// обрабатываем только строки с выводом прогресса и выводим в консоль
		if strings.Contains(line, "[download]") && strings.Contains(line, "%") {
			fmt.Print(Yellow + "\ryt-dlp: " + Reset + line + "          ")
		}
	}

	fmt.Print("\n")

	// Если yt-dlp завершился с ошибкой, CloseWithError передаст её в scanner.Err()
	if err := scanner.Err(); err != nil {
		log.Printf("Ошибка при скачивании: %v", err)
		return downloadLog, err
	}

	return downloadLog, nil
}
