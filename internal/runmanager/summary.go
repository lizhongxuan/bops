package runmanager

import (
	"fmt"
	"strings"

	"bops/internal/state"
)

type EndSummary struct {
	Status       string   `json:"status"`
	TotalSteps   int      `json:"total_steps"`
	SuccessSteps int      `json:"success_steps"`
	FailedSteps  int      `json:"failed_steps"`
	DurationMs   int64    `json:"duration_ms"`
	Issues       []string `json:"issues,omitempty"`
	Message      string   `json:"message,omitempty"`
}

func BuildEndSummary(run state.RunState) EndSummary {
	status := strings.TrimSpace(run.Status)
	if status == "" {
		status = "success"
	}

	summary := EndSummary{
		Status:     status,
		TotalSteps: len(run.Steps),
		Message:    run.Message,
	}

	if !run.StartedAt.IsZero() && !run.FinishedAt.IsZero() {
		summary.DurationMs = run.FinishedAt.Sub(run.StartedAt).Milliseconds()
	}

	issueSet := map[string]struct{}{}
	failedSteps := map[string]struct{}{}
	successSteps := 0

	addIssue := func(issue string) {
		issue = strings.TrimSpace(issue)
		if issue == "" {
			return
		}
		if _, ok := issueSet[issue]; ok {
			return
		}
		issueSet[issue] = struct{}{}
		summary.Issues = append(summary.Issues, issue)
	}

	for _, step := range run.Steps {
		stepFailed := step.Status == "failed"
		stepSucceeded := step.Status == "success"
		hostFailed := false
		for _, host := range step.Hosts {
			if host.Status != "failed" {
				continue
			}
			hostFailed = true
			failedSteps[step.Name] = struct{}{}
			msg := strings.TrimSpace(host.Message)
			if msg == "" {
				msg = "execution failed"
			}
			addIssue(fmt.Sprintf("%s@%s: %s", step.Name, host.Host, msg))
		}

		if stepFailed {
			failedSteps[step.Name] = struct{}{}
			if !hostFailed {
				addIssue(fmt.Sprintf("%s: failed", step.Name))
			}
		}

		if stepSucceeded && !hostFailed {
			successSteps++
		}
	}

	if run.Message != "" {
		addIssue(run.Message)
	}

	summary.FailedSteps = len(failedSteps)
	summary.SuccessSteps = successSteps

	return summary
}
