package main

import (
	"log"
	"os"
	"path/filepath"

	"telegramBotTrade/internal/config"
	"telegramBotTrade/internal/server"
	"telegramBotTrade/internal/storage"
	"telegramBotTrade/internal/telegram"
)

func main() {
	cfg := config.Load()

	// Ensure parent directory for the DB exists
	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755)
	db, err := storage.OpenSQLite("file:" + cfg.DBPath + "?_fk=1")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Printf("db: opened sqlite at %s", cfg.DBPath)
	if err := storage.InitSchema(db); err != nil {
		log.Fatal(err)
	}
	log.Println("db: schema ensured (messages table)")

	tg, err := telegram.NewBot(cfg.TelegramToken, cfg.WebhookPublicURL, db, cfg.OpenAIKey)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("telegram: bot initialized, webhook target %s", cfg.WebhookPublicURL)

	mux := server.NewHTTPMux(tg.WebhookHandler) // registers /telegram/webhook
	addr := ":" + cfg.Port
	log.Println("http: listening on", addr)
	if err := server.ListenAndServe(addr, mux); err != nil {
		log.Println("server error:", err)
		os.Exit(1)
	}
}
