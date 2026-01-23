package ai

import (
	"context"
	"fmt"

	"bops/internal/logging"
	"go.uber.org/zap"
)

type Client interface {
	Chat(ctx context.Context, messages []Message) (string, error)
}

type ThoughtClient interface {
	ChatWithThought(ctx context.Context, messages []Message) (string, string, error)
}

type StreamDelta struct {
	Content string
	Thought string
}

type StreamClient interface {
	ChatStream(ctx context.Context, messages []Message, onDelta func(StreamDelta)) (string, string, error)
}

type Config struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

func NewClient(cfg Config) (Client, error) {
	logging.L().Debug("ai client init",
		zap.String("provider", cfg.Provider),
		zap.String("base_url", cfg.BaseURL),
		zap.String("model", cfg.Model),
	)
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
