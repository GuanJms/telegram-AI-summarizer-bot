package telegram

import (
	"encoding/json"
	"net/http"

	"telegramBotTrade/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api   *tgbotapi.BotAPI
	store *storage.Store
	h     *Handlers
}

func NewBot(token, webhookURL string, db storage.DB, openAIKey string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	// set webhook
	webhook, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		return nil, err
	}
	if _, err := api.Request(webhook); err != nil {
		return nil, err
	}

	s := storage.NewStore(db)
	h := NewHandlers(api, s, openAIKey)

	return &Bot{api: api, store: s, h: h}, nil
}

// Webhook HTTP handler (registered at /telegram/webhook)
func (b *Bot) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "bad update", 400)
		return
	}
	if update.Message != nil {
		go b.h.HandleMessage(update.Message)
	}
	w.WriteHeader(http.StatusOK)
}
