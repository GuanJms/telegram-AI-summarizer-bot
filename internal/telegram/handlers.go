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
	// /stock SYMBOL [1d|1w|1m]
	reStock = regexp.MustCompile(`^/stock(?:@[\w_]+)?\s+([A-Za-z0-9\.^_=+-]+)(?:\s+(1d|1w|1m))?$`)
	// /stocks S1 S2 ... [1d|1w|1m]
	reStocks = regexp.MustCompile(`^/stocks(?:@[\w_]+)?\s+([A-Za-z0-9\.^_=+\-\s]+?)(?:\s+(1d|1w|1m))?$`)
	// /help
	reHelp = regexp.MustCompile(`^/(help|start)(?:@[\w_]+)?$`)
	// /stocks-index S1 S2 ... [interval] [window]
	// interval one of 1m|5m|15m|1h|1d, window e.g. 1d|5d|1m|3m|6m|1y|2y|5y|10y|30y
	reStocksIndex = regexp.MustCompile(`^/stocks-index(?:@[\w_]+)?\s+([A-Za-z0-9\.^_=+\-\s]+?)(?:\s+(1m|5m|15m|1h|1d))?(?:\s+(1d|5d|1m|3m|6m|1y|2y|5y|10y|30y))?$`)
	// /stockx SYMBOL [interval] [window]
	reStockX = regexp.MustCompile(`^/stockx(?:@[\w_]+)?\s+([A-Za-z0-9\.^_=+-]+)(?:\s+(1m|5m|15m|1h|1d))?(?:\s+(1d|5d|1m|3m|6m|1y|2y|5y|10y|30y))?$`)
	// /stocksx S1 S2 ... [interval] [window]
	reStocksX = regexp.MustCompile(`^/stocksx(?:@[\w_]+)?\s+([A-Za-z0-9\.^_=+\-\s]+?)(?:\s+(1m|5m|15m|1h|1d))?(?:\s+(1d|5d|1m|3m|6m|1y|2y|5y|10y|30y))?$`)
	// /ew-port S1 S2 ... [Xd|Xw|Xm|Xy] - Equal weighted portfolio backtest
	reEWPort = regexp.MustCompile(`^/ew-port(?:@[\w_]+)?\s+([A-Za-z0-9\.^_=+\-\s]+?)(?:\s+(\d+[dwmy]))?$`)
	// /port S1 X1 S2 X2 ... Y - Weighted portfolio backtest
	rePort = regexp.MustCompile(`^/port(?:@[\w_]+)?\s+(.+)$`)
	// /recommend TEXT - Trading recommendation based on user input
	reRecommend = regexp.MustCompile(`^/recommend(?:@[\w_]+)?\s+(.+)$`)
)

type Handlers struct {
	api       *tgbotapi.BotAPI
	store     *storage.Store
	summarize *openai.Summarizer
	recommend *openai.Recommender
}

func NewHandlers(api *tgbotapi.BotAPI, store *storage.Store, openAIKey string) *Handlers {
	return &Handlers{
		api:       api,
		store:     store,
		summarize: openai.NewSummarizer(openAIKey),
		recommend: openai.NewRecommender(openAIKey),
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
		g := reStock.FindStringSubmatch(txt)
		sym := g[1]
		window := ""
		if len(g) >= 3 {
			window = g[2]
		}
		h.handleStock(m.Chat.ID, sym, window)

	case reHelp.MatchString(txt):
		// Show commands help
		h.handleHelp(m.Chat.ID)

	case reStocks.MatchString(txt):
		g := reStocks.FindStringSubmatch(txt)
		symsField := strings.TrimSpace(g[1])
		window := ""
		if len(g) >= 3 {
			window = g[2]
		}
		// Split on whitespace, normalize and dedupe
		raw := strings.Fields(symsField)
		seen := map[string]struct{}{}
		syms := make([]string, 0, len(raw))
		for _, s := range raw {
			su := strings.ToUpper(strings.TrimSpace(s))
			if su == "" {
				continue
			}
			if _, ok := seen[su]; ok {
				continue
			}
			seen[su] = struct{}{}
			syms = append(syms, su)
		}
		if len(syms) < 2 {
			h.reply(m.Chat.ID, "Please provide at least two symbols, e.g. /stocks SPY AAPL 1w")
			return
		}
		h.handleMultiStock(m.Chat.ID, syms, window)

	case reStocksIndex.MatchString(txt):
		g := reStocksIndex.FindStringSubmatch(txt)
		symsField := strings.TrimSpace(g[1])
		interval := "5m"
		if len(g) >= 3 && g[2] != "" {
			interval = g[2]
		}
		window := ""
		if len(g) >= 4 {
			window = g[3]
		}
		raw := strings.Fields(symsField)
		seen := map[string]struct{}{}
		syms := make([]string, 0, len(raw))
		for _, s := range raw {
			su := strings.ToUpper(strings.TrimSpace(s))
			if su == "" {
				continue
			}
			if _, ok := seen[su]; ok {
				continue
			}
			seen[su] = struct{}{}
			syms = append(syms, su)
		}
		if len(syms) < 2 {
			h.reply(m.Chat.ID, "Please provide at least two symbols, e.g. /stocks-index SPY AAPL 1h 1y")
			return
		}
		img, err := finance.MakeIndexedChart(syms, interval, window, true)
		if err != nil {
			h.reply(m.Chat.ID, "Indexed plot failed: "+err.Error())
			return
		}
		name := strings.Join(syms, "_")
		photo := tgbotapi.NewPhoto(m.Chat.ID, tgbotapi.FileBytes{Name: name + "_indexed.png", Bytes: img})
		photo.Caption = "Indexed: " + strings.Join(syms, ", ") + " • " + strings.ToUpper(interval) + " • " + strings.ToUpper(window)
		h.api.Send(photo)

	case reStockX.MatchString(txt):
		g := reStockX.FindStringSubmatch(txt)
		sym := g[1]
		interval := "5m"
		if len(g) >= 3 && g[2] != "" {
			interval = g[2]
		}
		window := ""
		if len(g) >= 4 {
			window = g[3]
		}
		img, err := finance.MakeChart(sym, interval, window)
		if err != nil {
			h.reply(m.Chat.ID, "Chart failed: "+err.Error())
			return
		}
		photo := tgbotapi.NewPhoto(m.Chat.ID, tgbotapi.FileBytes{Name: sym + "_" + interval + "_" + window + ".png", Bytes: img})
		photo.Caption = strings.ToUpper(sym) + " • " + strings.ToUpper(interval) + " • " + strings.ToUpper(window)
		h.api.Send(photo)

	case reStocksX.MatchString(txt):
		g := reStocksX.FindStringSubmatch(txt)
		symsField := strings.TrimSpace(g[1])
		interval := "5m"
		if len(g) >= 3 && g[2] != "" {
			interval = g[2]
		}
		window := ""
		if len(g) >= 4 {
			window = g[3]
		}
		raw := strings.Fields(symsField)
		seen := map[string]struct{}{}
		syms := make([]string, 0, len(raw))
		for _, s := range raw {
			su := strings.ToUpper(strings.TrimSpace(s))
			if su == "" {
				continue
			}
			if _, ok := seen[su]; ok {
				continue
			}
			seen[su] = struct{}{}
			syms = append(syms, su)
		}
		if len(syms) < 2 {
			h.reply(m.Chat.ID, "Please provide at least two symbols, e.g. /stocksx SPY AAPL 1h 1y")
			return
		}
		img, err := finance.MakeMultiChart(syms, interval, window)
		if err != nil {
			h.reply(m.Chat.ID, "Multi chart failed: "+err.Error())
			return
		}
		name := strings.Join(syms, "_")
		photo := tgbotapi.NewPhoto(m.Chat.ID, tgbotapi.FileBytes{Name: name + "_" + interval + "_" + window + ".png", Bytes: img})
		photo.Caption = "Multi: " + strings.Join(syms, ", ") + " • " + strings.ToUpper(interval) + " • " + strings.ToUpper(window)
		h.api.Send(photo)

	case reEWPort.MatchString(txt):
		g := reEWPort.FindStringSubmatch(txt)
		symsField := strings.TrimSpace(g[1])
		window := "1y" // Default to 1 year
		if len(g) >= 3 && g[2] != "" {
			window = g[2]
		}
		raw := strings.Fields(symsField)
		seen := map[string]struct{}{}
		syms := make([]string, 0, len(raw))
		for _, s := range raw {
			su := strings.ToUpper(strings.TrimSpace(s))
			if su == "" {
				continue
			}
			if _, ok := seen[su]; ok {
				continue
			}
			seen[su] = struct{}{}
			syms = append(syms, su)
		}
		if len(syms) < 2 {
			h.reply(m.Chat.ID, "Please provide at least two symbols, e.g. /ew-port SPY AAPL QQQ 2y")
			return
		}
		h.handlePortfolio(m.Chat.ID, syms, window)

	case rePort.MatchString(txt):
		g := rePort.FindStringSubmatch(txt)
		input := strings.TrimSpace(g[1])

		symbols, weights, window, err := finance.ParseWeightedPortfolio(input)
		if err != nil {
			h.reply(m.Chat.ID, fmt.Sprintf("Invalid portfolio format: %v\n\nUsage: /port SPY 0.5 AAPL 0.25 1y", err))
			return
		}
		if len(symbols) == 0 {
			h.reply(m.Chat.ID, "Please provide at least one symbol with weight, e.g. /port SPY 0.6 AAPL 0.3 1y")
			return
		}
		h.handleWeightedPortfolio(m.Chat.ID, symbols, weights, window)

	case reRecommend.MatchString(txt):
		g := reRecommend.FindStringSubmatch(txt)
		userInput := strings.TrimSpace(g[1])
		if userInput == "" {
			h.reply(m.Chat.ID, "Please provide your investment thesis or market view after /recommend")
			return
		}
		h.reply(m.Chat.ID, "🤖 Analyzing your request and generating trading recommendations...")
		h.handleRecommendation(m.Chat.ID, userInput)
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

func (h *Handlers) handleStock(chatID int64, sym string, window string) {
	img, err := finance.Make5mChart(sym, window)
	if err != nil {
		h.reply(chatID, fmt.Sprintf("Couldn’t fetch %s: %v", sym, err))
		return
	}
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: sym + ".png", Bytes: img})
	w := strings.ToLower(strings.TrimSpace(window))
	if w == "" {
		w = "1d"
	}
	photo.Caption = strings.ToUpper(sym) + " • 5m • " + strings.ToUpper(w)
	h.api.Send(photo)
}

func (h *Handlers) handleMultiStock(chatID int64, syms []string, window string) {
	img, err := finance.MakeMulti5mChart(syms, window)
	if err != nil {
		h.reply(chatID, fmt.Sprintf("Couldn’t fetch multi: %v", err))
		return
	}
	name := strings.Join(syms, "_")
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: name + ".png", Bytes: img})
	w := strings.ToLower(strings.TrimSpace(window))
	if w == "" {
		w = "1d"
	}
	photo.Caption = "Multi: " + strings.Join(syms, ", ") + " • 5m • " + strings.ToUpper(w)
	h.api.Send(photo)
}

func (h *Handlers) handlePortfolio(chatID int64, syms []string, window string) {
	img, err := finance.MakePortfolioChart(syms, window)
	if err != nil {
		h.reply(chatID, fmt.Sprintf("Portfolio failed: %v", err))
		return
	}
	name := strings.Join(syms, "_")
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: name + "_portfolio_" + window + ".png", Bytes: img})
	photo.Caption = "Equal Weighted Portfolio: " + strings.Join(syms, ", ") + " • " + strings.ToUpper(window)
	h.api.Send(photo)
}

func (h *Handlers) handleWeightedPortfolio(chatID int64, syms []string, weights []float64, window string) {
	img, err := finance.MakeWeightedPortfolioChart(syms, weights, window)
	if err != nil {
		h.reply(chatID, fmt.Sprintf("Weighted portfolio failed: %v", err))
		return
	}

	// Create descriptive filename and caption
	var weightStrs []string
	for i, symbol := range syms {
		weightStrs = append(weightStrs, fmt.Sprintf("%s%.1f", symbol, weights[i]*100))
	}

	name := strings.Join(weightStrs, "_")
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: name + "_wport_" + window + ".png", Bytes: img})

	// Calculate total weight and cash
	totalWeight := 0.0
	for _, w := range weights {
		totalWeight += w
	}
	cashPct := (1.0 - totalWeight) * 100

	var caption strings.Builder
	caption.WriteString("Weighted Portfolio: ")
	for i, symbol := range syms {
		if i > 0 {
			caption.WriteString(", ")
		}
		weight := weights[i]
		if weight >= 0 {
			caption.WriteString(fmt.Sprintf("%s %.1f%%", symbol, weight*100))
		} else {
			caption.WriteString(fmt.Sprintf("%s %.1f%% SHORT", symbol, -weight*100))
		}
	}
	if cashPct > 0 {
		caption.WriteString(fmt.Sprintf(", Cash %.1f%%", cashPct))
	} else if cashPct < 0 {
		caption.WriteString(fmt.Sprintf(", Margin %.1f%%", -cashPct))
	}
	caption.WriteString(" • " + strings.ToUpper(window))

	photo.Caption = caption.String()
	h.api.Send(photo)
}

func (h *Handlers) handleHelp(chatID int64) {
	help := "Commands\n\n" +
		"- /summary [hours] - Summarize chat messages from the last N hours (default: 1, max: 48)\n" +
		"- /recommend TEXT - Get AI-powered trading recommendations based on your market view or thesis\n" +
		"- /stock SYMBOL [1d|1w|1m] - Single-symbol 5m mini chart\n" +
		"- /stocks S1 S2 ... [1d|1w|1m] - Multi-symbol 5m; auto-normalizes to % when >2\n" +
		"- /stockx SYMBOL [1m|5m|15m|1h|1d] [1d|5d|1m|3m|6m|1y|2y|5y|10y|30y] - Single-symbol custom\n" +
		"- /stocksx S1 S2 ... [interval] [window] - Multi-symbol custom; auto-normalizes to % when >2\n" +
		"- /stocks-index S1 S2 ... [interval] [window] - Index to base 100 at start for relative performance\n" +
		"- /ew-port S1 S2 ... [Xd|Xw|Xm|Xy] - Equal weighted portfolio backtest (starting $100)\n" +
		"- /port S1 W1 S2 W2 ... [Xd|Xw|Xm|Xy] - Weighted portfolio (W>0=long, W<0=short, rest=cash/margin)\n" +
		"\nLimits (Yahoo): 1m→30d, 5m→90d, 15m→180d, 1h→2y, 1d→30y. X-axis in Eastern Time."
	h.reply(chatID, help)
}

func (h *Handlers) handleRecommendation(chatID int64, userInput string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	recommendation, err := h.recommend.GetTradingRecommendation(ctx, userInput)
	if err != nil {
		h.reply(chatID, "Failed to generate recommendation: "+err.Error())
		return
	}

	msg := tgbotapi.NewMessage(chatID, recommendation)
	msg.ParseMode = "Markdown"
	h.api.Send(msg)
}

func (h *Handlers) reply(chatID int64, text string) {
	h.api.Send(tgbotapi.NewMessage(chatID, text))
}
