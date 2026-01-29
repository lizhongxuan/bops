package runmanager

import (
	"testing"
	"time"

	"bops/internal/state"
)

func TestBuildEndSummary(t *testing.T) {
	started := time.Now().Add(-2 * time.Second)
	finished := time.Now()

	run := state.RunState{
		Status:     "failed",
		Message:    "run failed",
		StartedAt:  started,
		FinishedAt: finished,
		Steps: []state.StepState{
			{
				Name:   "step-1",
				Status: "success",
				Hosts: map[string]state.HostResult{
					"h1": {Host: "h1", Status: "success"},
				},
			},
			{
				Name:   "step-2",
				Status: "failed",
				Hosts: map[string]state.HostResult{
					"h2": {Host: "h2", Status: "failed", Message: "boom"},
				},
			},
			{
				Name:   "step-3",
				Status: "running",
			},
		},
	}

	summary := BuildEndSummary(run)
	if summary.Status != "failed" {
		t.Fatalf("expected status failed, got %q", summary.Status)
	}
	if summary.TotalSteps != 3 {
		t.Fatalf("expected total steps 3, got %d", summary.TotalSteps)
	}
	if summary.SuccessSteps != 1 {
		t.Fatalf("expected success steps 1, got %d", summary.SuccessSteps)
	}
	if summary.FailedSteps != 1 {
		t.Fatalf("expected failed steps 1, got %d", summary.FailedSteps)
	}
	if summary.DurationMs <= 0 {
		t.Fatalf("expected duration ms > 0, got %d", summary.DurationMs)
	}
	if summary.Message != "run failed" {
		t.Fatalf("expected message run failed, got %q", summary.Message)
	}
	if !containsIssue(summary.Issues, "step-2@h2: boom") {
		t.Fatalf("expected issue step-2@h2: boom, got %+v", summary.Issues)
	}
	if !containsIssue(summary.Issues, "run failed") {
		t.Fatalf("expected issue run failed, got %+v", summary.Issues)
	}
}

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if issue == want {
			return true
		}
	}
	return false
}
