package aiworkflow

import (
	"context"
	"strings"
	"sync"
	"testing"

	"bops/internal/ai"
)

type smartClient struct {
	mu            sync.Mutex
	intentJSON    string
	planJSON      string
	workflowJSON  string
	fixJSON       string
	reviewReplies []string
	reviewCalls   int
}

func (c *smartClient) Chat(_ context.Context, messages []ai.Message) (string, error) {
	if len(messages) == 0 {
		return c.workflowJSON, nil
	}
	content := messages[len(messages)-1].Content
	switch {
	case strings.Contains(content, "workflow YAML reviewer"):
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.reviewCalls < len(c.reviewReplies) {
			reply := c.reviewReplies[c.reviewCalls]
			c.reviewCalls++
			return reply, nil
		}
		return `{"issues":[]}`, nil
	case strings.Contains(content, "Fix the YAML below"):
		if c.fixJSON != "" {
			return c.fixJSON, nil
		}
		return c.workflowJSON, nil
	case strings.Contains(content, "intent_type"):
		return c.intentJSON, nil
	case strings.Contains(content, "step_name"):
		return c.planJSON, nil
	default:
		return c.workflowJSON, nil
	}
}

func TestMultiAgentParallel(t *testing.T) {
	client := &smartClient{
		intentJSON:   `{"intent_type":"explain","goal":"install nginx","missing":[]}`,
		planJSON:     `[{"step_name":"install","description":"install nginx","dependencies":[]}]`,
		workflowJSON: `{"workflow":{"version":"v0.1","name":"demo","description":"","inventory":{"hosts":{"local":{"address":"127.0.0.1"}}},"plan":{"mode":"manual-approve","strategy":"sequential"},"steps":[{"name":"install","action":"cmd.run","with":{"cmd":"echo hi"}}]}}`,
	}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	specs := []AgentSpec{
		{Name: "main", Role: "primary"},
		{Name: "reviewer", Role: "qa"},
	}
	client.reviewReplies = []string{`{"issues":[]}`}
	state, err := pipeline.RunMultiAgent(context.Background(), "install nginx", nil, specs, RunOptions{SkipExecute: true})
	if err != nil {
		t.Fatalf("run multi agent: %v", err)
	}
	if state == nil || len(state.SubagentSummaries) != 1 {
		t.Fatalf("expected one subagent summary, got %+v", state.SubagentSummaries)
	}
	if state.SubagentSummaries[0].AgentName != "reviewer" {
		t.Fatalf("expected reviewer summary, got %+v", state.SubagentSummaries[0])
	}
}

func TestMultiAgentSummaryMerge(t *testing.T) {
	client := &smartClient{
		intentJSON:   `{"intent_type":"explain","goal":"install nginx","missing":[]}`,
		planJSON:     `[{"step_name":"install","description":"install nginx","dependencies":[]}]`,
		workflowJSON: `{"workflow":{"version":"v0.1","name":"demo","description":"","inventory":{"hosts":{"local":{"address":"127.0.0.1"}}},"plan":{"mode":"manual-approve","strategy":"sequential"},"steps":[{"name":"install","action":"cmd.run","with":{"cmd":"echo hi"}}]}}`,
		fixJSON:      `{"workflow":{"version":"v0.1","name":"demo","description":"","inventory":{"hosts":{"local":{"address":"127.0.0.1"}}},"plan":{"mode":"manual-approve","strategy":"sequential"},"steps":[{"name":"install","action":"cmd.run","with":{"cmd":"echo fixed"}}]}}`,
		reviewReplies: []string{
			`{"issues":["missing step description"]}`,
			`{"issues":[]}`,
		},
	}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	specs := []AgentSpec{
		{Name: "main", Role: "primary"},
		{Name: "reviewer", Role: "qa"},
		{Name: "coder", Role: "coder"},
		{Name: "helper", Role: "assistant"},
	}
	state, err := pipeline.RunMultiAgent(context.Background(), "install nginx", nil, specs, RunOptions{SkipExecute: true})
	if err != nil {
		t.Fatalf("run multi agent: %v", err)
	}
	if len(state.Issues) != 0 {
		t.Fatalf("expected issues cleared, got %v", state.Issues)
	}
	foundReviewer := false
	foundHelper := false
	for _, summary := range state.SubagentSummaries {
		switch summary.AgentName {
		case "reviewer":
			foundReviewer = true
			if !strings.Contains(summary.Summary, "issues=0") {
				t.Fatalf("expected reviewer summary issues=0, got %s", summary.Summary)
			}
		case "helper":
			foundHelper = true
		}
	}
	if !foundReviewer || !foundHelper {
		t.Fatalf("expected reviewer and helper summaries, got %+v", state.SubagentSummaries)
	}
}
