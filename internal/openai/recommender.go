package openai

import (
	"context"
	"fmt"

	oa "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Recommender struct {
	cli oa.Client
}

func NewRecommender(apiKey string) *Recommender {
	client := oa.NewClient(option.WithAPIKey(apiKey))
	return &Recommender{cli: client}
}

func (r *Recommender) GetTradingRecommendation(ctx context.Context, userInput string) (string, error) {
	systemPrompt := `You are a professional financial analyst providing structured trading recommendations. You will receive a user's investment thesis or market view and provide a comprehensive analysis.

Your response must follow this exact structure:

**Interpretation:**
[Explain what this bet means in market terms]

**Ticker Recommendations:**
[List specific ETFs, indices, or instruments to go long/short with clear direction]

**Rationale:**
[For each ticker, explain what it tracks and why it gains/loses if the thesis plays out]

**Risks:**
[Describe scenarios where the trade would lose money]

Guidelines:
- Focus on liquid, widely available instruments (ETFs, major indices)
- Provide specific ticker symbols (e.g., SPY, QQQ, TLT, etc.)
- Be clear about long vs short positions
- Consider both direct and indirect ways to play the thesis
- Include risk management perspective
- Use clear, concise explanations
- Format with bullet points where appropriate`

	userPrompt := fmt.Sprintf("User wants to bet on: %s\n\nProvide trading recommendations following the structured format.", userInput)

	resp, err := r.cli.Chat.Completions.New(ctx, oa.ChatCompletionNewParams{
		Model: "gpt-4",
		Messages: []oa.ChatCompletionMessageParamUnion{
			oa.SystemMessage(systemPrompt),
			oa.UserMessage(userPrompt),
		},
		MaxTokens: oa.Int(1500), // Limit response length for telegram
	})
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
