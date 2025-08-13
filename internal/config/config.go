package config

import (
	"log"
	"os"
)

type Config struct {
	TelegramToken    string
	WebhookPublicURL string
	OpenAIKey        string
	Port             string
	DBPath           string
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing env %s", k)
	}
	return v
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9095"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/app/data/chat.db"
	}
	return Config{
		TelegramToken:    mustEnv("TELEGRAM_BOT_TOKEN"),
		WebhookPublicURL: mustEnv("WEBHOOK_PUBLIC_URL"),
		OpenAIKey:        mustEnv("OPENAI_API_KEY"),
		Port:             port,
		DBPath:           dbPath,
	}
}
