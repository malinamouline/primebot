package main

import (
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"hard90-bot/internal/bot"
	"hard90-bot/internal/config"
	"hard90-bot/internal/storage"
)

func main() {
	cfg := config.Load()

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Fatalf("timezone: %v", err)
	}

	store, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer store.Close()

	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	api.Debug = false
	log.Printf("bot @%s started (tz: %s)", api.Self.UserName, cfg.Timezone)

	handler := bot.NewHandler(api, store, loc)
	reminder := bot.NewReminderService(handler, store, loc, cfg.ReminderHour, cfg.ReminderMinute)
	reminder.Start()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := api.GetUpdatesChan(u)
	for update := range updates {
		handler.HandleUpdate(update)
	}
}
