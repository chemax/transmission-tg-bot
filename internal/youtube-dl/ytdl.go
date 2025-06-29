package youtube_dl

import (
	"context"
	"errors"
	"fmt"
	"github.com/lrstanley/go-ytdlp"
	"path/filepath"
	"strings"
	"transmission-tg-control/internal/cfg"
)

type YoutubeDL struct {
	proxy         string
	downloadPath  string
	ytDlpLocation string
	ctx           context.Context
}

func New(c cfg.Youtube) *YoutubeDL {
	return &YoutubeDL{
		ytDlpLocation: c.YtDlpLocation,
		proxy:         c.Proxy,
		downloadPath:  c.DownloadPath,
	}
}

func (y *YoutubeDL) InsertCtx(ctx context.Context) {
	y.ctx = ctx
}

func (y *YoutubeDL) Download(url string, callback func(err error)) {
	ytdlp.MustInstall(y.ctx, nil)

	if !strings.Contains(url, "youtube.com") && !strings.Contains(url, "youtu.be") {
		callback(errors.New("not youtube url"))
		return
	}
	fileName := fmt.Sprintf("%s.mp4", strings.ReplaceAll(url, "/", "_"))
	outputPath := filepath.Join(y.downloadPath, fileName)
	go func() {
		callback(y.downloadVideo(outputPath, url))
	}()
}

func (y *YoutubeDL) downloadVideo(outputPath, url string) error {
	dl := ytdlp.New().BreakOnExisting().
		FormatSort("res,ext:mp4:m4a").
		NoProgress().
		NoPlaylist().
		NoOverwrites().
		Continue().
		RecodeVideo("mp4").
		Output("%(title)s.%(ext)s").
		Proxy(y.proxy)

	_, err := dl.Run(y.ctx, url)
	if err != nil {
		fmt.Println(err.Error())
	}
	return err
}
