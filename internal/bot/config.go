package bot

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// цвета для терминала
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

var commandsConfig = tgbotapi.SetMyCommandsConfig{
	Commands: []tgbotapi.BotCommand{
		{
			Command:     "info",
			Description: "Информация о боте",
		},
	},
}

type BotCfg struct {
	Token       string `toml:"bot_token"`
	InfoText    string `toml:"bot_infotext"`
	SongQuality string `toml:"song_quality"`
}

// NewBotCfg создает шаблонный конфиг бота
func NewBotCfg() *BotCfg {
	return &BotCfg{
		Token:       "Your bot token",
		InfoText:    "",
		SongQuality: "default",
	}
}

// Load загружает конфиг бота из файла config.toml в созданый [BotCfg].
// Если файл отсутствует ([os.ErrNotExist]), метод автоматически создаёт новый с дефолтными значениями.
//
// # Пример использования:
//
//	cfg := NewBotCfg()
//	if err := cfg.Load(); err != nil {
//	    log.Fatal(err)
//	}
func (cfg *BotCfg) Load() error {

	// загрузка конфига
	_, err := toml.DecodeFile("./config.toml", cfg)

	// если файла нет
	if errors.Is(err, os.ErrNotExist) {

		log.Printf(Red + "config.toml не найден, будет создан новый" + Reset)

		// Шаблон конфига с комментариями
		defaultContent := `# Токен бота от @BotFather
			token = "Your bot token"

			# Текст сообщения для команды /info
			info_text = ""

			# Качество звука: "default" или "max"
			song_quality = "default"
			`

		// Записываем дефолтный конфиг прямо в файл
		if err := os.WriteFile("./config.toml", []byte(defaultContent), 0644); err != nil {
			log.Printf(Red + "Не удалось создать новый config.toml" + Reset)
			return err
		}

		return fmt.Errorf("config.toml создан, заполните его и перезапустите бота")
	}

	// Если файл есть, но он битый или нет прав на чтение
	if err != nil {
		log.Printf(Red+"Не удалось прочитать config.toml"+Reset+": %v", err)
		return err
	}

	return nil
}
