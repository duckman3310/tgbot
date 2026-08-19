принимает ссылку на видео ютуб, скачивает аудио через yt-dlp и отправляет в чат в .m4a 

| Команда / Флаг | Описание |  
| :--- | :--- |  
| ссылка | скачивает и отправляет аудио | 
| `-l` | Отправляет лог работы yt-dlp | 
| `/info` | Отправляет инфу о боте | 

---

| Зависимости | Описание | Как скачать |
| :--- | :--- | :--- |  
| [Go](https://go.dev/) (1.21+) | сам go, тут понятно |
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | yt-dlp скачивает видео с ютуб | забыл
| [telegram-bot-api](github.com/go-telegram-bot-api/telegram-bot-api/v5) | api для бота, скачиваем через | `go get -u github.com/go-telegram-bot-api/telegram-bot-api/v5`
| **ffmpeg** | иногда тг воспринимает форматы типо .opus как голосовухи, на всякие случай перед отправкой все конвертируется в .m4a | должен быть по дефолту
| **Node.js** | тут не шарю, но джемини говорит что для скачивания yt-dlp нужно проходить js ребусы, для этого js и скачиваем | хз, с офиц сайта
| **кукисы** | тут тоже, но чтоб проблем у yt-dlp не было скармливаем ему куки (файл cookies.txt кидаем в корневую папку проэкта)| Получить их можно через например [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)