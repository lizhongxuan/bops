package ai

import (
	"context"
	"fmt"
)

type Client interface {
	Chat(ctx context.Context, messages []Message) (string, error)
}

type Config struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "", "none":
		return nil, fmt.Errorf("ai provider is not configured")
	case "openai":
		return newOpenAIClient(cfg), nil
	case "deepseek":
		return newDeepseekClient(cfg), nil
	case "gemini":
		return newGeminiClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", cfg.Provider)
	}
}
