package aiworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"bops/internal/ai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type adkModelOptions struct {
	streamSink StreamSink
}

func withADKStreamSink(sink StreamSink) model.Option {
	return model.WrapImplSpecificOptFn(func(opts *adkModelOptions) {
		opts.streamSink = sink
	})
}

type adkModelAdapter struct {
	client ai.Client
	tools  []*schema.ToolInfo
}

func newADKModelAdapter(client ai.Client) model.ToolCallingChatModel {
	return &adkModelAdapter{client: client}
}

func (m *adkModelAdapter) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	clone := *m
	clone.tools = tools
	return &clone, nil
}

func (m *adkModelAdapter) Generate(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	messages := toAIMessages(input)
	if len(m.tools) > 0 {
		messages = injectToolInstruction(messages, m.tools)
	}
	if client, ok := m.client.(ai.ThoughtClient); ok {
		reply, thought, err := client.ChatWithThought(ctx, messages)
		if err != nil {
			return nil, err
		}
		if len(m.tools) > 0 {
			if msg, ok := buildToolCallMessage(reply, thought, m.tools); ok {
				return msg, nil
			}
		}
		return &schema.Message{
			Role:             schema.Assistant,
			Content:          strings.TrimSpace(reply),
			ReasoningContent: strings.TrimSpace(thought),
		}, nil
	}
	reply, err := m.client.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	if len(m.tools) > 0 {
		if msg, ok := buildToolCallMessage(reply, "", m.tools); ok {
			return msg, nil
		}
	}
	return &schema.Message{
		Role:    schema.Assistant,
		Content: strings.TrimSpace(reply),
	}, nil
}

func (m *adkModelAdapter) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if len(m.tools) > 0 {
		msg, err := m.Generate(ctx, input, opts...)
		if err != nil {
			return nil, err
		}
		return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
	}

	messages := toAIMessages(input)
	sinkOpts := model.GetImplSpecificOptions(&adkModelOptions{}, opts...)
	if client, ok := m.client.(ai.StreamClient); ok {
		reader, writer := schema.Pipe[*schema.Message](0)
		var closed atomic.Bool
		go func() {
			defer writer.Close()
			_, _, err := client.ChatStream(ctx, messages, func(delta ai.StreamDelta) {
				if sinkOpts.streamSink != nil {
					sinkOpts.streamSink(delta)
				}
				if delta.Content == "" && delta.Thought == "" {
					return
				}
				if closed.Load() {
					return
				}
				if writer.Send(&schema.Message{
					Role:             schema.Assistant,
					Content:          delta.Content,
					ReasoningContent: delta.Thought,
				}, nil) {
					closed.Store(true)
				}
			})
			if err != nil {
				writer.Send(nil, err)
			}
		}()
		return reader, nil
	}

	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func toAIMessages(input []*schema.Message) []ai.Message {
	if len(input) == 0 {
		return nil
	}
	out := make([]ai.Message, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			continue
		}
		content := msg.Content
		if content == "" && len(msg.UserInputMultiContent) > 0 {
			content = msg.UserInputMultiContent[0].Text
		}
		out = append(out, ai.Message{Role: string(msg.Role), Content: content})
	}
	return out
}

func toSchemaMessagePtrs(messages []ai.Message) []*schema.Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		role := schema.RoleType(strings.TrimSpace(msg.Role))
		if role == "" {
			role = schema.User
		}
		out = append(out, &schema.Message{Role: role, Content: msg.Content})
	}
	return out
}

func injectToolInstruction(messages []ai.Message, tools []*schema.ToolInfo) []ai.Message {
	if len(tools) == 0 {
		return messages
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool == nil || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		names = append(names, tool.Name)
	}
	if len(names) == 0 {
		return messages
	}
	instruction := fmt.Sprintf(
		"You must respond with JSON only. If you need to call a tool, respond as: "+
			`{\"tool\":\"%s\",\"args\":{...}}`+
			". If you are answering the user directly, respond as: "+
			`{\"final\":\"...\"}. `+
			"Available tools: %s.",
		strings.Join(names, "|"),
		strings.Join(names, ", "),
	)
	if len(messages) == 0 {
		return []ai.Message{{Role: "system", Content: instruction}}
	}
	if strings.EqualFold(messages[0].Role, "system") {
		messages[0].Content = strings.TrimSpace(messages[0].Content + "\n\n" + instruction)
		return messages
	}
	return append([]ai.Message{{Role: "system", Content: instruction}}, messages...)
}

type toolCallEnvelope struct {
	Tool      string          `json:"tool,omitempty"`
	ToolName  string          `json:"tool_name,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Final     string          `json:"final,omitempty"`
	Response  string          `json:"response,omitempty"`
}

func buildToolCallMessage(reply string, thought string, tools []*schema.ToolInfo) (*schema.Message, bool) {
	envelope, ok := parseToolEnvelope(reply, tools)
	if !ok {
		if fallbackArgs, ok := tryBuildSingleToolArgs(reply, tools); ok {
			callID := uuid.NewString()
			return &schema.Message{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID: callID,
					Function: schema.FunctionCall{
						Name:      fallbackArgs.toolName,
						Arguments: fallbackArgs.arguments,
					},
					Type: "function",
				}},
				ReasoningContent: strings.TrimSpace(thought),
			}, true
		}
		return nil, false
	}
	if envelope.Final != "" || envelope.Response != "" {
		finalText := envelope.Final
		if finalText == "" {
			finalText = envelope.Response
		}
		return &schema.Message{
			Role:             schema.Assistant,
			Content:          strings.TrimSpace(finalText),
			ReasoningContent: strings.TrimSpace(thought),
		}, true
	}
	toolName := envelope.Tool
	if toolName == "" {
		toolName = envelope.ToolName
	}
	if toolName == "" {
		return nil, false
	}
	args := envelope.Args
	if len(args) == 0 {
		args = envelope.Arguments
	}
	argText := strings.TrimSpace(string(args))
	if argText == "" {
		argText = "{}"
	}
	callID := uuid.NewString()
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID: callID,
			Function: schema.FunctionCall{
				Name:      toolName,
				Arguments: argText,
			},
			Type: "function",
		}},
		ReasoningContent: strings.TrimSpace(thought),
	}, true
}

func parseToolEnvelope(reply string, tools []*schema.ToolInfo) (toolCallEnvelope, bool) {
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		jsonText = strings.TrimSpace(reply)
	}
	var envelope toolCallEnvelope
	if err := json.Unmarshal([]byte(jsonText), &envelope); err != nil {
		return toolCallEnvelope{}, false
	}
	toolName := strings.TrimSpace(envelope.Tool)
	if toolName == "" {
		toolName = strings.TrimSpace(envelope.ToolName)
	}
	if toolName == "" && envelope.Final == "" && envelope.Response == "" {
		return toolCallEnvelope{}, false
	}
	if toolName != "" && !toolNameAllowed(toolName, tools) {
		return toolCallEnvelope{}, false
	}
	return envelope, true
}

type fallbackToolArgs struct {
	toolName  string
	arguments string
}

func tryBuildSingleToolArgs(reply string, tools []*schema.ToolInfo) (fallbackToolArgs, bool) {
	if len(tools) != 1 {
		return fallbackToolArgs{}, false
	}
	toolName := strings.TrimSpace(tools[0].Name)
	if toolName == "" || !strings.EqualFold(toolName, "step_patch") {
		return fallbackToolArgs{}, false
	}
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		jsonText = strings.TrimSpace(reply)
	}
	if jsonText == "" || !strings.HasPrefix(strings.TrimSpace(jsonText), "{") {
		return fallbackToolArgs{}, false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return fallbackToolArgs{}, false
	}
	if _, ok := payload["step_name"]; !ok {
		if _, ok := payload["action"]; !ok {
			return fallbackToolArgs{}, false
		}
	}
	return fallbackToolArgs{toolName: toolName, arguments: jsonText}, true
}

func toolNameAllowed(name string, tools []*schema.ToolInfo) bool {
	if name == "" {
		return false
	}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if strings.EqualFold(tool.Name, name) {
			return true
		}
	}
	return false
}
