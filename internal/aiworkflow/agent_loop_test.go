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
	final := `{"action":"final","yaml":"version: v0.1\nname: demo\ndescription: ''\ninventory:\n  hosts:\n    local:\n      address: 127.0.0.1\nplan:\n  mode: manual-approve\n  strategy: sequential\nsteps:\n  - name: step1\n    action: cmd.run\n    args:\n      cmd: \"echo hi\"\n"}`
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
	workflowJSON := `{"workflow":{"version":"v0.1","name":"demo","steps":[{"name":"install","action":"cmd.run","args":{"cmd":"echo hi"}}]}}`
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

func TestAgentLoopRalphStopHookBlocksThenCompletes(t *testing.T) {
	final := `{"action":"final","yaml":"version: v0.1\nname: demo\nsteps:\n  - name: step1\n    action: cmd.run\n    args:\n      cmd: \"echo hi\"\n"}`
	tool := `{"action":"tool_call","tool":"test_runner","args":{"cmd":"go test ./..."}}`
	client := &loopClient{responses: []string{final, tool, final}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	events := []Event{}
	state, err := pipeline.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		RalphMode:        true,
		LoopProfile:      "ralph",
		CompletionChecks: []string{"tests_green", "has_steps"},
		LoopMaxIters:     5,
		NoProgressLimit:  2,
		EventSink: func(evt Event) {
			events = append(events, evt)
		},
		ToolExecutor: func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return "PASS", nil
		},
		ToolNames: []string{"test_runner"},
	})
	if err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if state.LoopMetrics == nil {
		t.Fatalf("expected loop metrics")
	}
	if state.LoopMetrics.Terminal != LoopTerminationCompleted {
		t.Fatalf("expected completed terminal reason, got %s", state.LoopMetrics.Terminal)
	}
	if state.LoopMetrics.Iterations != 3 {
		t.Fatalf("expected 3 iterations, got %d", state.LoopMetrics.Iterations)
	}
	blocked := 0
	for _, evt := range events {
		if evt.EventType == "completion_blocked" {
			blocked++
		}
	}
	if blocked == 0 {
		t.Fatalf("expected completion_blocked event")
	}
}

func TestAgentLoopRalphNoProgress(t *testing.T) {
	final := `{"action":"final","yaml":"version: v0.1\nname: demo\nsteps:\n  - name: step1\n    action: cmd.run\n    args:\n      cmd: \"echo hi\"\n"}`
	client := &loopClient{responses: []string{final, final, final}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	state, err := pipeline.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		RalphMode:       true,
		CompletionToken: "<promise>COMPLETE</promise>",
		LoopMaxIters:    5,
		NoProgressLimit: 1,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "no progress") {
		t.Fatalf("expected no progress error, got %v", err)
	}
	if state == nil || state.LoopMetrics == nil {
		t.Fatalf("expected loop metrics on no progress")
	}
	if state.LoopMetrics.Terminal != LoopTerminationNoProgress {
		t.Fatalf("expected no_progress terminal reason, got %s", state.LoopMetrics.Terminal)
	}
}

func TestAgentLoopRalphPersistentMemoryResume(t *testing.T) {
	root := t.TempDir()
	tool := `{"action":"tool_call","tool":"test_runner","args":{"cmd":"go test ./..."}}`
	final := `{"action":"final","yaml":"version: v0.1\nname: demo\nsteps:\n  - name: step1\n    action: cmd.run\n    args:\n      cmd: \"echo hi\"\n"}`

	client1 := &loopClient{responses: []string{tool, final}}
	pipeline1, err := New(Config{Client: client1, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline1: %v", err)
	}
	_, err = pipeline1.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		RalphMode:        true,
		LoopProfile:      "ralph",
		LoopMemoryRoot:   root,
		SessionKey:       "resume-case",
		CompletionChecks: []string{"tests_green", "has_steps"},
		LoopMaxIters:     4,
		ToolExecutor: func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return "PASS", nil
		},
		ToolNames: []string{"test_runner"},
	})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	client2 := &loopClient{responses: []string{final}}
	pipeline2, err := New(Config{Client: client2, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline2: %v", err)
	}
	state, err := pipeline2.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		RalphMode:          true,
		LoopProfile:        "ralph",
		LoopMemoryRoot:     root,
		ResumeCheckpointID: "resume-case",
		CompletionChecks:   []string{"tests_green", "has_steps"},
		LoopMaxIters:       2,
	})
	if err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if state.LoopMetrics == nil {
		t.Fatalf("expected loop metrics")
	}
	if state.LoopMetrics.SessionID != "resume-case" {
		t.Fatalf("expected session_id resume-case, got %s", state.LoopMetrics.SessionID)
	}
	if state.LoopMetrics.NonDurable {
		t.Fatalf("expected durable memory mode")
	}
	if state.LoopMetrics.Terminal != LoopTerminationCompleted {
		t.Fatalf("expected completed terminal reason, got %s", state.LoopMetrics.Terminal)
	}
}

func TestAgentLoopRalphFallbackToInMemoryWarning(t *testing.T) {
	final := `{"action":"final","yaml":"version: v0.1\nname: demo\nsteps:\n  - name: step1\n    action: cmd.run\n    args:\n      cmd: \"echo hi\"\n"}`
	client := &loopClient{responses: []string{final}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	warnings := 0
	state, err := pipeline.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		RalphMode: true,
		EventSink: func(evt Event) {
			if evt.EventType == "loop_memory_warning" {
				warnings++
			}
		},
	})
	if err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if state.LoopMetrics == nil {
		t.Fatalf("expected loop metrics")
	}
	if !state.LoopMetrics.NonDurable {
		t.Fatalf("expected non-durable memory mode")
	}
	if warnings == 0 {
		t.Fatalf("expected loop memory warning event")
	}
}

func TestAgentLoopBudgetExceededByToolCalls(t *testing.T) {
	tool := `{"action":"tool_call","tool":"noop","args":{}}`
	client := &loopClient{responses: []string{tool}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	state, err := pipeline.RunAgentLoop(context.Background(), "install", nil, RunOptions{
		LoopMaxIters: 4,
		MaxToolCalls: 1,
		ToolExecutor: func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return "ok", nil
		},
		ToolNames: []string{"noop"},
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "budget exceeded") {
		t.Fatalf("expected budget exceeded error, got %v", err)
	}
	if state == nil || state.LoopMetrics == nil {
		t.Fatalf("expected loop metrics")
	}
	if state.LoopMetrics.Terminal != LoopTerminationBudgetExceeded {
		t.Fatalf("expected budget_exceeded terminal reason, got %s", state.LoopMetrics.Terminal)
	}
}
