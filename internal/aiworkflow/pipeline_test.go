package aiworkflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bops/internal/ai"
	"bops/internal/validationenv"
	"bops/internal/validationrun"
)

type fakeClient struct {
	responses []string
	idx       int
}

func (f *fakeClient) Chat(_ context.Context, _ []ai.Message) (string, error) {
	if f.idx >= len(f.responses) {
		return "", errors.New("no response configured")
	}
	resp := f.responses[f.idx]
	f.idx++
	return resp, nil
}

func TestPipelineGenerateFixExecute(t *testing.T) {
	intentJSON := `{"goal":"install nginx","missing":[]}`
	planJSON := `[{"step_name":"install","description":"install nginx","dependencies":[]}]`
	badJSON := `{"workflow":{"version":"v0.1","name":"demo","steps":[{"name":"install","args":{"cmd":"echo hi"}}]}}`
	goodJSON := `{"workflow":{"version":"v0.1","name":"demo","steps":[{"name":"install","action":"cmd.run","args":{"cmd":"echo hi"}}]}}`

	client := &fakeClient{responses: []string{intentJSON, planJSON, badJSON, goodJSON}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	called := 0
	originalRunner := validationrun.Runner
	validationrun.Runner = func(_ context.Context, _ validationenv.ValidationEnv, _ string) (validationrun.Result, error) {
		called++
		return validationrun.Result{Status: "success"}, nil
	}
	t.Cleanup(func() {
		validationrun.Runner = originalRunner
	})

	env := &validationenv.ValidationEnv{Name: "test", Type: validationenv.EnvTypeContainer, Image: "dummy"}
	state, err := pipeline.RunGenerate(context.Background(), "install", nil, RunOptions{
		ValidationEnv: env,
		SkipExecute:   false,
	})
	if err != nil {
		t.Fatalf("run generate: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected execute to run once, got %d", called)
	}
	if state.ExecutionResult == nil || state.ExecutionResult.Status != "success" {
		t.Fatalf("expected execution result success, got %+v", state.ExecutionResult)
	}
	if len(state.Issues) != 0 {
		t.Fatalf("expected issues cleared, got %v", state.Issues)
	}
	if state.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", state.RetryCount)
	}
	if len(state.History) != 1 {
		t.Fatalf("expected history length 1, got %d", len(state.History))
	}
	if state.NeedsReview {
		t.Fatalf("expected needsReview false")
	}
}

func TestPipelineQuestionGate(t *testing.T) {
	intentJSON := `{"goal":"install nginx","missing":["targets","constraints"]}`
	client := &fakeClient{responses: []string{intentJSON}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	state, err := pipeline.RunGenerate(context.Background(), "install nginx", nil, RunOptions{
		SkipExecute: true,
	})
	if err != nil {
		t.Fatalf("run generate: %v", err)
	}
	if len(state.Questions) == 0 {
		t.Fatalf("expected questions to be populated")
	}
	if strings.TrimSpace(state.YAML) != "" {
		t.Fatalf("expected yaml to be empty when awaiting questions")
	}
	if state.NeedsReview {
		t.Fatalf("expected needsReview false")
	}
}
