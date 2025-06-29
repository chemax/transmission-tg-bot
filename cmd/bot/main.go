package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	youtube_dl "transmission-tg-control/internal/youtube-dl"

	"transmission-tg-control/internal/cfg"
	"transmission-tg-control/internal/tg"
	"transmission-tg-control/internal/tr"
)

func main() {
	conf, err := cfg.Load(os.Args[1])
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	trClient, err := tr.New(conf.RPC.URL, conf.RPC.User, conf.RPC.Password)
	if err != nil {
		log.Fatalf("transmission: %v", err)
	}
	ytClient := youtube_dl.New(conf.Youtube)

	bot, err := tg.New(conf, trClient, ytClient)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	// корректное завершение по SIGINT/SIGTERM
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		bot.Stop()
	}()

	log.Println("bot started")
	bot.Start()
}
