package aiworkflow

import (
	"context"
	"errors"
	"strings"

	"bops/internal/ai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
)

func runChatWithADK(ctx context.Context, client ai.Client, messages []ai.Message, sink StreamSink) (string, string, error) {
	if client == nil {
		return "", "", errors.New("ai client is not configured")
	}
	modelAdapter := newADKModelAdapter(client)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "bops-chat",
		Description:   "bops chat agent",
		Model:         modelAdapter,
		MaxIterations: 1,
	})
	if err != nil {
		return "", "", err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: sink != nil,
	})
	opts := []adk.AgentRunOption{}
	if sink != nil {
		opts = append(opts, adk.WithChatModelOptions([]model.Option{withADKStreamSink(sink)}))
	}
	iter := runner.Run(ctx, toSchemaMessagePtrs(messages), opts...)
	var replyBuilder strings.Builder
	var thoughtBuilder strings.Builder
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return "", "", event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return "", "", err
		}
		if msg == nil {
			continue
		}
		if msg.Content != "" {
			replyBuilder.WriteString(msg.Content)
		}
		if msg.ReasoningContent != "" {
			thoughtBuilder.WriteString(msg.ReasoningContent)
		}
	}
	return strings.TrimSpace(replyBuilder.String()), strings.TrimSpace(thoughtBuilder.String()), nil
}
