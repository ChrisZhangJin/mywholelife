package dream

import (
	"context"
	"errors"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// HookGen produces a single-line recall hook for a memory being consolidated.
// It is the LLM seam (D-02/D-03): production is llmHookGen; tests inject a fake
// so the dream job is hermetic. It returns only the hook text — the store's
// Compress(hook) seam assembles and defends the index line.
type HookGen interface {
	Hook(ctx context.Context, brief, body string) (string, error)
}

const hookSystemPrompt = `You generate a single-line recall hook for an AI agent's long-term-memory index.
Given a memory's brief and its SKILL.md body, output ONE concise phrase (max ~15
words) describing what this memory is and when the agent should recall it.
Rules: output ONLY the phrase. No markdown, no bullet, no quotes, no code fences,
no memory id, no newline, no leading dash. Never write prose paragraphs or summaries.`

type llmHookGen struct {
	client *openai.Client
	model  string
}

func newLLMHookGen(baseURL, apiKey, model string) *llmHookGen {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return &llmHookGen{client: openai.NewClientWithConfig(cfg), model: model}
}

func (g *llmHookGen) Hook(ctx context.Context, brief, body string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       g.model,
		Temperature: 0.2,
		MaxTokens:   60,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: hookSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: "Brief: " + brief + "\n\nSKILL.md:\n" + body + "\n\nHook:"},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errors.New("dream: empty hook completion")
	}
	return resp.Choices[0].Message.Content, nil
}
