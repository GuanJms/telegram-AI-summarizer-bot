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

	// Debug: Check if database file exists before opening
	if stat, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
		log.Printf("db: creating new database file at %s", cfg.DBPath)
	} else {
		log.Printf("db: using existing database file at %s (size: %d bytes, modified: %s)",
			cfg.DBPath, stat.Size(), stat.ModTime().Format("2006-01-02 15:04:05"))
	}

	// Debug: Check the data directory contents
	if files, err := os.ReadDir(filepath.Dir(cfg.DBPath)); err == nil {
		log.Printf("db: data directory contents:")
		for _, file := range files {
			info, _ := file.Info()
			log.Printf("  - %s (size: %d, modified: %s)", file.Name(), info.Size(), info.ModTime().Format("2006-01-02 15:04:05"))
		}
	}

	db, err := storage.OpenSQLite("file:" + cfg.DBPath + "?_fk=1")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Printf("db: opened sqlite at %s", cfg.DBPath)
	if err := storage.InitSchema(db); err != nil {
		log.Fatal(err)
	}
	log.Println("db: schema ensured (messages and command_usage tables)")

	// Debug: Check existing data count
	store := storage.NewStore(db)
	if stats, err := store.FetchUsageStats(0); err == nil {
		totalCommands := 0
		for _, stat := range stats {
			totalCommands += stat.Count
		}
		log.Printf("db: found %d existing command usage records", totalCommands)
	}

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
