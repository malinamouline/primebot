package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken  string
	Timezone       string
	DBPath         string
	ReminderHour   int
	ReminderMinute int
}

func Load() *Config {
	envPaths := []string{
		".env",
		"../.env",
		"../../.env",
	}
	loaded := false
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			loaded = true
			break
		}
	}
	if !loaded {
		log.Println("no .env file found, reading from environment")
	}

	return &Config{
		TelegramToken:  mustEnv("TELEGRAM_BOT_TOKEN"),
		Timezone:       getEnv("TIMEZONE", "Europe/Moscow"),
		DBPath:         getEnv("DB_PATH", "./data/summer83.db"),
		ReminderHour:   getEnvInt("REMINDER_HOUR", 21),
		ReminderMinute: getEnvInt("REMINDER_MINUTE", 0),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("invalid %s=%q, using %d", key, v, fallback)
		return fallback
	}
	return n
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}
