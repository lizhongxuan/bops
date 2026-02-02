package aiworkflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bops/internal/ai"
)

type loopClient struct {
	responses []string
	idx       int
}

func (f *loopClient) Chat(_ context.Context, _ []ai.Message) (string, error) {
	if f.idx >= len(f.responses) {
		return "", errors.New("no response configured")
	}
	resp := f.responses[f.idx]
	f.idx++
	return resp, nil
}

func TestAgentLoopToolCallFinal(t *testing.T) {
	toolCall := `{"action":"tool_call","tool":"search_file","args":{"pattern":"config/*.json"}}`
	final := `{"action":"final","yaml":"version: v0.1\nname: demo\ndescription: ''\ninventory:\n  hosts:\n    local:\n      address: 127.0.0.1\nplan:\n  mode: manual-approve\n  strategy: sequential\nsteps:\n  - name: step1\n    action: cmd.run\n    with:\n      cmd: \"echo hi\"\n"}`
	client := &loopClient{responses: []string{toolCall, final}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	state, err := pipeline.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		ToolExecutor: func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return "found config", nil
		},
		ToolNames:    []string{"search_file"},
		LoopMaxIters: 4,
	})
	if err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if strings.TrimSpace(state.YAML) == "" {
		t.Fatalf("expected yaml output")
	}
	if state.LoopMetrics == nil {
		t.Fatalf("expected loop metrics")
	}
	if state.LoopMetrics.Iterations != 2 {
		t.Fatalf("expected iterations 2, got %d", state.LoopMetrics.Iterations)
	}
	if state.LoopMetrics.ToolCalls != 1 {
		t.Fatalf("expected tool calls 1, got %d", state.LoopMetrics.ToolCalls)
	}
	if state.LoopMetrics.ToolFailures != 0 {
		t.Fatalf("expected tool failures 0, got %d", state.LoopMetrics.ToolFailures)
	}
}

func TestAgentLoopMaxIters(t *testing.T) {
	toolCall := `{"action":"tool_call","tool":"noop","args":{}}`
	client := &loopClient{responses: []string{toolCall, toolCall, toolCall}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	state, err := pipeline.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		ToolExecutor: func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return "ok", nil
		},
		ToolNames:    []string{"noop"},
		LoopMaxIters: 2,
	})
	if err == nil || !strings.Contains(err.Error(), "max iterations") {
		t.Fatalf("expected max iterations error, got %v", err)
	}
	if state == nil || state.LoopMetrics == nil {
		t.Fatalf("expected metrics even on failure")
	}
	if state.LoopMetrics.Iterations != 2 {
		t.Fatalf("expected iterations 2, got %d", state.LoopMetrics.Iterations)
	}
}

func TestAgentLoopFallbackToPipeline(t *testing.T) {
	invalid := `{"action":"tool_call"}`
	intentJSON := `{"goal":"install nginx","missing":[]}`
	workflowJSON := `{"workflow":{"version":"v0.1","name":"demo","steps":[{"name":"install","action":"cmd.run","with":{"cmd":"echo hi"}}]}}`
	client := &loopClient{responses: []string{invalid, invalid, intentJSON, workflowJSON}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	state, err := pipeline.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		FallbackToPipeline: true,
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if strings.TrimSpace(state.YAML) == "" {
		t.Fatalf("expected yaml from fallback pipeline")
	}
}
