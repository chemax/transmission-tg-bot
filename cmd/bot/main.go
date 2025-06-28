package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"transmission-tg-control/internal/cfg"
	"transmission-tg-control/internal/tg"
	"transmission-tg-control/internal/tr"
)

func main() {
	conf, err := cfg.Load("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	trClient, err := tr.New(conf.RPC.URL, conf.RPC.User, conf.RPC.Password)
	if err != nil {
		log.Fatalf("transmission: %v", err)
	}

	bot, err := tg.New(conf, trClient)
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
