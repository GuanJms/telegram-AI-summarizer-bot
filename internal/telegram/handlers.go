package telegram

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegramBotTrade/internal/finance"
	"telegramBotTrade/internal/openai"
	"telegramBotTrade/internal/storage"
)

var (
	reSummary = regexp.MustCompile(`^/summary(?:@[\w_]+)?(?:\s+|/)?(\d+)?$`)
	reStock   = regexp.MustCompile(`^/stock(?:@[\w_]+)?\s+([A-Za-z0-9\.^_=+-]+)$`)
)

type Handlers struct {
	api       *tgbotapi.BotAPI
	store     *storage.Store
	summarize *openai.Summarizer
}

func NewHandlers(api *tgbotapi.BotAPI, store *storage.Store, openAIKey string) *Handlers {
	return &Handlers{
		api:       api,
		store:     store,
		summarize: openai.NewSummarizer(openAIKey),
	}
}

func (h *Handlers) HandleMessage(m *tgbotapi.Message) {
	// Save any text for later summaries
	if txt := strings.TrimSpace(m.Text); txt != "" {
		_ = h.store.SaveMessage(m.Chat.ID, m.From.ID, txt, int64(m.Date))
	}

	txt := strings.TrimSpace(m.Text)
	switch {
	case reSummary.MatchString(txt):
		hours := 1
		if g := reSummary.FindStringSubmatch(txt); len(g) == 2 && g[1] != "" {
			fmt.Sscanf(g[1], "%d", &hours)
			if hours < 1 {
				hours = 1
			}
			if hours > 48 {
				hours = 48
			}
		}
		h.reply(m.Chat.ID, fmt.Sprintf("Summarizing last %dh…", hours))
		h.handleSummary(m.Chat.ID, hours)

	case reStock.MatchString(txt):
		sym := reStock.FindStringSubmatch(txt)[1]
		h.handleStock(m.Chat.ID, sym)
	}
}

func (h *Handlers) handleSummary(chatID int64, hours int) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	msgs, err := h.store.FetchMessages(chatID, since)
	if err != nil {
		h.reply(chatID, "Summary failed: "+err.Error())
		return
	}
	if len(msgs) == 0 {
		h.reply(chatID, "No messages found in the selected time window.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	out, err := h.summarize.Summarize(ctx, msgs)
	if err != nil {
		h.reply(chatID, "Summary failed: "+err.Error())
		return
	}
	msg := tgbotapi.NewMessage(chatID, out)
	msg.ParseMode = "Markdown"
	h.api.Send(msg)
}

func (h *Handlers) handleStock(chatID int64, sym string) {
	img, err := finance.Make5mChart(sym)
	if err != nil {
		h.reply(chatID, fmt.Sprintf("Couldn’t fetch %s: %v", sym, err))
		return
	}
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: sym + ".png", Bytes: img})
	photo.Caption = strings.ToUpper(sym) + " • 5-minute mini chart"
	h.api.Send(photo)
}

func (h *Handlers) reply(chatID int64, text string) {
	h.api.Send(tgbotapi.NewMessage(chatID, text))
}
