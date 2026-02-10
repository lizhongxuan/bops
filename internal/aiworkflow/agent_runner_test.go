package aiworkflow

import (
	"context"
	"errors"
	"testing"

	"bops/internal/ai"
)

type agentRunnerClient struct {
	responses []string
	idx       int
}

func (f *agentRunnerClient) Chat(_ context.Context, _ []ai.Message) (string, error) {
	if f.idx >= len(f.responses) {
		return "", errors.New("no response configured")
	}
	resp := f.responses[f.idx]
	f.idx++
	return resp, nil
}

func TestRunAgentSetsAgentFields(t *testing.T) {
	intentJSON := `{"intent_type":"debug","goal":"install nginx","missing":[]}`
	planJSON := `[{"step_name":"install","description":"install nginx","dependencies":[]}]`
	goodJSON := `{"workflow":{"version":"v0.1","name":"demo","description":"","inventory":{"hosts":{"local":{"address":"127.0.0.1"}}},"plan":{"mode":"manual-approve","strategy":"sequential"},"steps":[{"name":"install","action":"cmd.run","args":{"cmd":"echo hi"}}]}}`
	client := &agentRunnerClient{responses: []string{intentJSON, planJSON, goodJSON}}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	var seen Event
	state, err := pipeline.RunAgent(context.Background(), "install", nil, RunOptions{
		SkipExecute: true,
		EventSink: func(evt Event) {
			if seen.Node == "" {
				seen = evt
			}
		},
		AgentSpec: AgentSpec{Name: "coder", Role: "worker"},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if state.AgentName != "coder" || state.AgentRole != "worker" {
		t.Fatalf("unexpected agent state: %+v", state)
	}
	if seen.AgentName != "coder" || seen.AgentRole != "worker" {
		t.Fatalf("unexpected agent event: %+v", seen)
	}
}
