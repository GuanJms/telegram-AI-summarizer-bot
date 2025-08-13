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
	if err := storage.InitSchema(db); err != nil {
		log.Fatal(err)
	}

	tg, err := telegram.NewBot(cfg.TelegramToken, cfg.WebhookPublicURL, db, cfg.OpenAIKey)
	if err != nil {
		log.Fatal(err)
	}

	mux := server.NewHTTPMux(tg.WebhookHandler) // registers /telegram/webhook
	addr := ":" + cfg.Port
	log.Println("listening on", addr)
	if err := server.ListenAndServe(addr, mux); err != nil {
		log.Println("server error:", err)
		os.Exit(1)
	}
}
