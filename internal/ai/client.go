package ai

import (
	"context"
	"fmt"
	"strings"

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
	Provider      string
	APIKey        string
	BaseURL       string
	Model         string
	PlannerModel  string
	ExecutorModel string
}

func NewClient(cfg Config) (Client, error) {
	logging.L().Debug("ai client init",
		zap.String("provider", cfg.Provider),
		zap.String("base_url", cfg.BaseURL),
		zap.String("model", cfg.Model),
	)
	if strings.TrimSpace(cfg.Provider) == "" || cfg.Provider == "none" {
		return nil, fmt.Errorf("ai provider is not configured")
	}

	if strings.TrimSpace(cfg.PlannerModel) == "" && strings.TrimSpace(cfg.ExecutorModel) == "" {
		return newProviderClient(cfg)
	}

	baseClient, err := newProviderClient(cfg)
	if err != nil {
		return nil, err
	}

	plannerCfg := cfg
	if strings.TrimSpace(cfg.PlannerModel) != "" {
		plannerCfg.Model = cfg.PlannerModel
	}
	executorCfg := cfg
	if strings.TrimSpace(cfg.ExecutorModel) != "" {
		executorCfg.Model = cfg.ExecutorModel
	}

	plannerClient, err := newProviderClient(plannerCfg)
	if err != nil {
		return nil, err
	}
	executorClient, err := newProviderClient(executorCfg)
	if err != nil {
		return nil, err
	}

	defaultClient := baseClient
	if strings.TrimSpace(cfg.ExecutorModel) != "" {
		defaultClient = executorClient
	}
	return NewRoutedClient(defaultClient, plannerClient, executorClient), nil
}

func newProviderClient(cfg Config) (Client, error) {
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
