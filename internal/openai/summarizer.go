package openai

import (
	"context"
	"regexp"
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
	// sanitize messages: strip URLs, markdown images, and non-textual blobs
	msgs := sanitizeMessages(messages)
	if len(msgs) == 0 {
		return "No text messages to summarize.", nil
	}
	// chunk to keep tokens reasonable
	const chunk = 60
	var partials []string
	for i := 0; i < len(msgs); i += chunk {
		end := i + chunk
		if end > len(msgs) {
			end = len(messages)
		}
		part := strings.Join(msgs[i:end], "\n")

		resp, err := s.cli.Chat.Completions.New(ctx, oa.ChatCompletionNewParams{
			Model: "gpt-4",
			Messages: []oa.ChatCompletionMessageParamUnion{
				oa.SystemMessage("You are a concise text-only chat summarizer. Ignore images, videos, stickers, audio, locations, code attachments, and links. Do not include or describe media. Use bullets. Capture decisions, questions, and action items (who/what/when)."),
				oa.UserMessage("Summarize this group chat excerpt concisely (text only):\n" + part),
			},
		})
		if err != nil {
			return "", err
		}
		partials = append(partials, resp.Choices[0].Message.Content)
	}

	merged := strings.Join(partials, "\n\n")
	final, err := s.cli.Chat.Completions.New(ctx, oa.ChatCompletionNewParams{
		Model: "gpt-4",
		Messages: []oa.ChatCompletionMessageParamUnion{
			oa.SystemMessage("Create a single compact text-only summary with sections: Key Points, Decisions, Open Questions, Action Items (Owner → Task → When). Do not include links or media descriptions."),
			oa.UserMessage(merged),
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(final.Choices[0].Message.Content), nil
}

var (
	reMarkdownImg = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`) // ![alt](url)
	reURL         = regexp.MustCompile(`https?://\S+`)
)

// sanitizeMessages removes media references and large non-textual content
func sanitizeMessages(messages []string) []string {
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		text := reMarkdownImg.ReplaceAllString(m, "")
		text = reURL.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		// cap individual message length to avoid huge blobs
		if len(text) > 2000 {
			text = text[:2000]
		}
		out = append(out, text)
	}
	return out
}
