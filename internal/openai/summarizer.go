package openai

import (
	"context"
	"strings"

	oa "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Summarizer struct {
	cli oa.Client
}

func NewSummarizer(apiKey string) *Summarizer {
	client := oa.NewClient(option.WithAPIKey(apiKey))
	return &Summarizer{cli: client}
}

func (s *Summarizer) Summarize(ctx context.Context, messages []string) (string, error) {
	// chunk to keep tokens reasonable
	const chunk = 60
	var partials []string
	for i := 0; i < len(messages); i += chunk {
		end := i + chunk
		if end > len(messages) {
			end = len(messages)
		}
		part := strings.Join(messages[i:end], "\n")

		resp, err := s.cli.Chat.Completions.New(ctx, oa.ChatCompletionNewParams{
			Model: oa.ChatModelGPT4oMini,
			Messages: []oa.ChatCompletionMessageParamUnion{
				oa.SystemMessage("You are a concise chat summarizer. Use bullets. Capture decisions, questions, and action items (who/what/when)."),
				oa.UserMessage("Summarize this group chat excerpt concisely:\n" + part),
			},
		})
		if err != nil {
			return "", err
		}
		partials = append(partials, resp.Choices[0].Message.Content)
	}

	merged := strings.Join(partials, "\n\n")
	final, err := s.cli.Chat.Completions.New(ctx, oa.ChatCompletionNewParams{
		Model: oa.ChatModelGPT4oMini,
		Messages: []oa.ChatCompletionMessageParamUnion{
			oa.SystemMessage("Create a single compact summary with sections: Key Points, Decisions, Open Questions, Action Items (Owner → Task → When)."),
			oa.UserMessage(merged),
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(final.Choices[0].Message.Content), nil
}
