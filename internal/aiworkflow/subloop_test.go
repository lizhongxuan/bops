package aiworkflow

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return string(data)
}

func TestSubLoopRetriesUntilReviewPasses(t *testing.T) {
	fragment1 := "- name: install\n  action: cmd.run\n  args:\n    cmd: echo hi"
	fragment2 := "- name: install\n  action: cmd.run\n  args:\n    cmd: echo ok"
	responses := []string{
		mustJSON(t, map[string]string{"yaml_fragment": fragment1}),
		mustJSON(t, map[string][]string{"issues": {"missing guardrail"}}),
		mustJSON(t, map[string]string{"yaml_fragment": fragment2}),
		mustJSON(t, map[string][]string{"issues": {}}),
	}
	client := &fakeClient{responses: responses}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	step := PlanStep{ID: "step-1", StepName: "install"}
	result, err := pipeline.runSubLoop(context.Background(), step, &State{}, RunOptions{})
	if err != nil {
		t.Fatalf("run subloop: %v", err)
	}
	if result.Rounds != 2 {
		t.Fatalf("expected rounds 2, got %d", result.Rounds)
	}
	if !strings.Contains(result.YAMLFragment, "echo ok") {
		t.Fatalf("expected latest fragment, got %q", result.YAMLFragment)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %v", result.Issues)
	}
}

func TestSubLoopReturnsIssuesAfterMaxRounds(t *testing.T) {
	fragment1 := "- name: install\n  action: cmd.run\n  args:\n    cmd: echo hi"
	fragment2 := "- name: install\n  action: cmd.run\n  args:\n    cmd: echo still"
	responses := []string{
		mustJSON(t, map[string]string{"yaml_fragment": fragment1}),
		mustJSON(t, map[string][]string{"issues": {"missing guardrail"}}),
		mustJSON(t, map[string]string{"yaml_fragment": fragment2}),
		mustJSON(t, map[string][]string{"issues": {"still failing"}}),
	}
	client := &fakeClient{responses: responses}
	pipeline, err := New(Config{Client: client, MaxRetries: 1})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	step := PlanStep{ID: "step-1", StepName: "install"}
	result, err := pipeline.runSubLoop(context.Background(), step, &State{}, RunOptions{})
	if err != nil {
		t.Fatalf("run subloop: %v", err)
	}
	if result.Rounds != 2 {
		t.Fatalf("expected rounds 2, got %d", result.Rounds)
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected issues to remain")
	}
	if !strings.Contains(strings.Join(result.Issues, ","), "still failing") {
		t.Fatalf("expected latest issues, got %v", result.Issues)
	}
}
