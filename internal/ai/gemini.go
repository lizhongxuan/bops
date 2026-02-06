package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bops/runner/logging"
	"go.uber.org/zap"
)

type geminiClient struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

const geminiRequestTimeout = 90 * time.Second

func newGeminiClient(cfg Config) *geminiClient {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gemini-1.5-pro"
	}
	return &geminiClient{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(base, "/"),
		model:   model,
		http:    &http.Client{Timeout: geminiRequestTimeout},
	}
}

func (c *geminiClient) Chat(ctx context.Context, messages []Message) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", fmt.Errorf("ai api key is required")
	}
	logging.L().Debug("ai chat request",
		zap.String("provider", "gemini"),
		zap.String("model", c.model),
		zap.Int("messages", len(messages)),
	)
	prompt := flattenMessages(messages)
	payload := geminiRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai request failed: %s", strings.TrimSpace(string(respBody)))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("ai response missing candidates")
	}

	text := parsed.Candidates[0].Content.Parts[0].Text
	logging.L().Debug("ai chat response",
		zap.String("provider", "gemini"),
		zap.Int("content_len", len(text)),
	)
	return text, nil
}

func flattenMessages(messages []Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		role := strings.ToUpper(strings.TrimSpace(msg.Role))
		if role == "" {
			role = "USER"
		}
		builder.WriteString(role)
		builder.WriteString(":\n")
		builder.WriteString(strings.TrimSpace(msg.Content))
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}
