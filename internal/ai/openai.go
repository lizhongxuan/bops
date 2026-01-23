package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bops/internal/logging"
	"go.uber.org/zap"
)

type openAIClient struct {
	apiKey     string
	baseURL    string
	model      string
	http       *http.Client
	streamHTTP *http.Client
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	Stream      bool            `json:"stream"` // 开启流式必须为 true
}

type openAIResponseMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// 2. 新增：流式响应的结构体 (Chunk)
type openAIStreamResponse struct {
	Choices []struct {
		Delta struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"` // DeepSeek 的思考过程
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIResponseMessage `json:"message"`
	} `json:"choices"`
}

const aiRequestTimeout = 90 * time.Second

func newOpenAIClient(cfg Config) *openAIClient {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &openAIClient{
		apiKey:     cfg.APIKey,
		baseURL:    strings.TrimRight(base, "/"),
		model:      model,
		http:       &http.Client{Timeout: aiRequestTimeout},
		streamHTTP: &http.Client{},
	}
}

func newDeepseekClient(cfg Config) *openAIClient {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = "https://api.deepseek.com/v1"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "deepseek-chat"
	}
	cfg.BaseURL = base
	cfg.Model = model
	return newOpenAIClient(cfg)
}

func (c *openAIClient) Chat(ctx context.Context, messages []Message) (string, error) {
	msg, err := c.doChat(ctx, messages)
	if err != nil {
		return "", err
	}
	return msg.Content, nil
}

func (c *openAIClient) ChatWithThought(ctx context.Context, messages []Message) (string, string, error) {
	msg, err := c.doChat(ctx, messages)
	if err != nil {
		return "", "", err
	}
	content := strings.TrimSpace(msg.Content)
	thought := strings.TrimSpace(msg.ReasoningContent)
	if thought == "" {
		content, thought = splitThoughtFromContent(content)
	}
	return content, thought, nil
}

func (c *openAIClient) doChat(ctx context.Context, messages []Message) (openAIResponseMessage, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return openAIResponseMessage{}, fmt.Errorf("ai api key is required")
	}

	logging.L().Debug("ai chat request",
		zap.String("provider", "openai"),
		zap.String("model", c.model),
		zap.Int("messages", len(messages)),
	)
	payload := openAIRequest{
		Model:       c.model,
		Messages:    toOpenAIMessages(messages),
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return openAIResponseMessage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return openAIResponseMessage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return openAIResponseMessage{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openAIResponseMessage{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return openAIResponseMessage{}, fmt.Errorf("ai request failed: %s", strings.TrimSpace(string(respBody)))
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return openAIResponseMessage{}, err
	}
	if len(parsed.Choices) == 0 {
		return openAIResponseMessage{}, fmt.Errorf("ai response missing choices")
	}

	msg := parsed.Choices[0].Message
	logging.L().Debug("ai chat response",
		zap.String("provider", "openai"),
		zap.Int("content_len", len(msg.Content)),
		zap.Int("thought_len", len(msg.ReasoningContent)),
	)
	return msg, nil
}

func toOpenAIMessages(messages []Message) []openAIMessage {
	result := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, openAIMessage{Role: msg.Role, Content: msg.Content})
	}
	return result
}

func splitThoughtFromContent(content string) (string, string) {
	start := strings.Index(content, "<think>")
	end := strings.Index(content, "</think>")
	if start == -1 || end == -1 || end <= start {
		return content, ""
	}
	thought := strings.TrimSpace(content[start+len("<think>") : end])
	cleaned := strings.TrimSpace(content[:start] + content[end+len("</think>"):])
	return cleaned, thought
}

// ChatStream 支持流式对话
// onDelta 会在每次收到内容或思考片段时被调用
func (c *openAIClient) ChatStream(ctx context.Context, messages []Message, onDelta func(StreamDelta)) (string, string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", "", fmt.Errorf("ai api key is required")
	}

	logging.L().Debug("ai stream chat request",
		zap.String("model", c.model),
	)

	// 1. 构造请求，开启 Stream
	payload := openAIRequest{
		Model:       c.model,
		Messages:    toOpenAIMessages(messages),
		Temperature: 0.2,
		Stream:      true, // 【关键】开启流式
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream") // 【关键】告诉服务端我们要流

	// 2. 发送请求 (注意：不要立刻 Close Body，要在读取完后 Close)
	client := c.streamHTTP
	if client == nil {
		client = c.http
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("ai stream request failed: %s", string(respBody))
	}

	// 3. 逐行读取 SSE 数据
	reader := bufio.NewReader(resp.Body)
	var contentBuilder strings.Builder
	var thoughtBuilder strings.Builder
	for {
		// 读取一行 (以 \n 结尾)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break // 读取完毕
			}
			return "", "", err
		}

		// 处理每一行数据
		lineStr := strings.TrimSpace(string(line))

		// SSE 格式通常以 "data: " 开头
		if !strings.HasPrefix(lineStr, "data:") {
			continue
		}

		// 去掉前缀
		dataStr := strings.TrimSpace(strings.TrimPrefix(lineStr, "data:"))

		// 结束标志
		if dataStr == "[DONE]" {
			break
		}

		// 解析 JSON chunk
		var chunk openAIStreamResponse
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			logging.L().Warn("unmarshal stream chunk failed", zap.Error(err), zap.String("data", dataStr))
			continue
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta

			// 如果有内容，调用回调函数传出去
			if delta.Content != "" || delta.ReasoningContent != "" {
				if delta.Content != "" {
					contentBuilder.WriteString(delta.Content)
				}
				if delta.ReasoningContent != "" {
					thoughtBuilder.WriteString(delta.ReasoningContent)
				}
				if onDelta != nil {
					onDelta(StreamDelta{Content: delta.Content, Thought: delta.ReasoningContent})
				}
			}
		}
	}

	content := strings.TrimSpace(contentBuilder.String())
	thought := strings.TrimSpace(thoughtBuilder.String())
	if thought == "" {
		content, thought = splitThoughtFromContent(content)
	}
	logging.L().Debug("ai stream chat response",
		zap.String("provider", "openai"),
		zap.Int("content_len", len(content)),
		zap.Int("thought_len", len(thought)),
	)
	return content, thought, nil
}
