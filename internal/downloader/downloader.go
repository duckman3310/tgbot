// Package downloader provides bot utilities for parse requests, download audio or audio info using yt-dlp
package downloader

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const fileLimit = 50 * 1024 * 1024 // 50мб

type scanResult struct {
	lines []string
	err   error
}

type readResult struct {
	data []byte
	err  error
}

type DownloadRequest struct {
	URL  string
	Args []string

	// DownloadSections is passed to yt-dlp as --download-sections value.
	// Format is "[*]start-end" in seconds or HH:MM:SS, e.g. "*0:30-1:00".
	// Empty string means the whole media is downloaded.
	DownloadSections string
}

type AudioInfo struct {
	Title    string
	Uploader string // channel name as reported by yt-dlp, not necessarily the artist
	Ext      string
}

type DownloadedAudio struct {
	Data      []byte // байты m4a-файла
	Title     string // название трека
	Performer string // исполнитель
	Thumbnail []byte // байты обложки (JPEG, до 200 КБ, до 320×320)
}

func Download(input string) (DownloadedAudio, error) {

	request := parseRequest(input)

	var audio DownloadedAudio

	if err := audio.fetchAudio(request); err != nil {
		return audio, err
	}

}

func (audio *DownloadedAudio) fetchAudio(request DownloadRequest) error {

	// Build yt-dlp and ffmpeg arguments
	downloadArgs := []string{
		"-f", "bestaudio[ext=m4a]/bestaudio[ext=mp4]",
		"-o", "-",
		"--no-playlist",
		"--no-progress",
		"--no-warnings",
		"--quiet",
		request.URL,
	}

	handleArgs := []string{
		"-i", "pipe:0",
		"-c:a", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov",
		"pipe:1",
	}

	// contexts
	downloadCtx, downloadCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer downloadCancel()

	handleCtx, handleCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer handleCancel()

	// commands
	downloadCmd := exec.CommandContext(downloadCtx, "yt-dlp", downloadArgs...)
	handleCmd := exec.CommandContext(handleCtx, "ffmpeg", handleArgs...)

	// err buffers
	var downloadErr bytes.Buffer
	downloadCmd.Stderr = &downloadErr

	var handleErr bytes.Buffer
	handleCmd.Stderr = &handleErr

	// out (audio file) buffer
	var out bytes.Buffer
	handleCmd.Stdout = &out

	// connecting download stdout and handle stdin pipes
	downloadStdout, err := downloadCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating download (yt-dlp) stdout pipe error: %w", err)
	}
	handleCmd.Stdin = downloadStdout

	// start commands (download must starts first)

	if err := downloadCmd.Start(); err != nil {
		return fmt.Errorf("start download (yt-dlp) error: %w", err)
	}

	if err := handleCmd.Start(); err != nil {
		return fmt.Errorf("start handle (ffmpeg) error: %w", err)
	}

	// Wait for both processes to finish
	handleWait := handleCmd.Wait()

	// If ffmpeg fails, cancel the context to kill yt-dlp immediately
	// instead of waiting for timeout.
	if handleWait != nil {
		downloadCancel()
		_ = downloadCmd.Wait()
		return fmt.Errorf("handle (ffmpeg) failed: %w error: %s", handleWait, handleErr.String())
	}

	downloadWait := downloadCmd.Wait()

	if downloadWait != nil {
		return fmt.Errorf("download (yt-dlp) failed: %w error: %s", downloadWait, downloadErr.String())
	}

	audio.Data = out.Bytes()
	return nil
}

func DownloadAudioInfo(ctx context.Context, url string) (AudioInfo, error) {

	var audioInfo AudioInfo

	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--no-progress",
		"--print", "title", // названия трека
		"--print", "uploader", // имени исполнителя
		"--print", "ext", // разшерение файла
		url,
	}

	// сборка вызова yt-dlp
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)

	// создание пайпов
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return audioInfo, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return audioInfo, err
	}

	// старт процесса
	if err := cmd.Start(); err != nil {
		return audioInfo, err
	}

	// гарантированное завершение процесса
	defer func() { _ = cmd.Process.Kill() }()

	// сканеры вывода и ошибок
	outScanner := bufio.NewScanner(stdout)
	errScanner := bufio.NewScanner(stderr)

	// каналы для получения результатов
	errCh := make(chan scanResult, 1)
	outCh := make(chan scanResult, 1)

	// чтение потоков
	go listenScanner(errCh, errScanner)
	go listenScanner(outCh, outScanner)

	// получение результатов
	errResult := <-errCh
	outResult := <-outCh

	// ожидания конца процесса
	cmdErr := cmd.Wait()

	// проверка ошибок чтения потоков
	if errResult.err != nil {
		return audioInfo, fmt.Errorf("чтение stderr не удалось: %w", errResult.err)
	}

	if outResult.err != nil {
		return audioInfo, fmt.Errorf("чтение stdout не удалось: %w", outResult.err)
	}

	// проверка ошибок выполнения yt-dlp
	if cmdErr != nil {
		return audioInfo, fmt.Errorf("yt-dlp завершился с ошибкой: %w; stderr: %s", cmdErr, strings.Join(errResult.lines, "\n"))
	}

	// сборка информации о треке
	if len(outResult.lines) < 2 {
		return audioInfo, errors.New("не удалось получить информацию о треке")
	}

	audioInfo.Title = outResult.lines[0]
	audioInfo.Uploader = outResult.lines[1]
	audioInfo.Ext = outResult.lines[2]

	// если yt-dlp завершен успешно, но с ошибками или варнингами
	if len(errResult.lines) > 0 {
		return audioInfo, fmt.Errorf("yt-dlp вернул ошибки: %s", strings.Join(errResult.lines, "\n"))
	}

	return audioInfo, nil
}

// DownloadAudio downloads a file as a byte slice using yt-dlp instead of
// writing it to disk. It returns an error if yt-dlp fails or if ctx times out.
//
// It may also return valid data together with an error if yt-dlp's failure
// was non-critical (e.g. warnings only).
func DownloadAudio(ctx context.Context, request DownloadRequest) ([]byte, error) {

	// сборка вызова
	args := []string{
		"-f", "bestaudio/best",
		"-o", "-",
		"--no-playlist",
		"--no-progress",
		"--no-warnings",
		"--quiet",
	}

	if request.DownloadSections != "" {
		args = append(args, "--download-sections", request.DownloadSections)
	}

	args = append(args, request.URL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)

	// создание пайпов
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	// вызов yt-dlp
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// гарантированное завершение
	defer func() { _ = cmd.Process.Kill() }()

	// сканер ошибок
	errScanner := bufio.NewScanner(stderr)
	errScanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// каналы для получения результатов
	errCh := make(chan scanResult, 1)
	outCh := make(chan readResult, 1)

	// чтение потоков
	go listenScanner(errCh, errScanner)
	go readBytes(outCh, stdout, fileLimit)

	// получение результатов
	errResult := <-errCh
	outResult := <-outCh

	// ожидания конца процесса
	cmdErr := cmd.Wait()

	// проверка ошибок чтения потоков
	if errResult.err != nil {
		return nil, fmt.Errorf("чтение stderr не удалось: %w", errResult.err)
	}

	if outResult.err != nil {
		return nil, fmt.Errorf("чтение stdout не удалось: %w", outResult.err)
	}

	// проверка ошибок выполнения yt-dlp
	if cmdErr != nil {
		return nil, fmt.Errorf("yt-dlp завершился с ошибкой: %w; stderr: %s", cmdErr, strings.Join(errResult.lines, "\n"))
	}

	// если yt-dlp завершен успешно, но с ошибками или варнингами
	if len(errResult.lines) > 0 {
		return outResult.data, fmt.Errorf("yt-dlp вернул ошибки: %s", strings.Join(errResult.lines, "\n"))
	}

	return outResult.data, nil
}

func listenScanner(resultCh chan<- scanResult, scanner *bufio.Scanner) {
	defer close(resultCh)

	var lines []string

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		resultCh <- scanResult{lines: lines, err: err}
		return
	}

	resultCh <- scanResult{lines: lines, err: nil}
}

func readBytes(resultCh chan<- readResult, r io.ReadCloser, limit int64) {
	defer close(resultCh)
	defer r.Close()

	lr := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		resultCh <- readResult{err: err}
		return
	}
	if int64(len(data)) > limit {
		resultCh <- readResult{err: fmt.Errorf("поток превысил лимит %d байт", limit)}
		return
	}
	resultCh <- readResult{data: data}
}

func parseRequest(input string) DownloadRequest {
	var request DownloadRequest

	for _, arg := range strings.Fields(input) {

		if arg == "" {
			continue
		}

		switch {
		case strings.HasPrefix(arg, "http://"), strings.HasPrefix(arg, "https://"):
			request.URL = arg

		case strings.HasPrefix(arg, "*"):
			request.DownloadSections = fmt.Sprintf("--download-sections %s", arg)
		}
	}

	return request
}
