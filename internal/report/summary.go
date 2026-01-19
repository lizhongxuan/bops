package report

import (
	"bops/internal/state"
	"time"
)

type Summary struct {
	RunID        string    `json:"run_id"`
	WorkflowName string    `json:"workflow_name"`
	Status       string    `json:"status"`
	FailedStep   string    `json:"failed_step,omitempty"`
	FailedHost   string    `json:"failed_host,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	Steps        int       `json:"steps"`
	FailedSteps  int       `json:"failed_steps"`
}

func Summarize(run state.RunState) Summary {
	summary := Summary{
		RunID:        run.RunID,
		WorkflowName: run.WorkflowName,
		Status:       "success",
		StartedAt:    run.StartedAt,
		FinishedAt:   run.FinishedAt,
		Steps:        len(run.Steps),
	}

	failedSteps := map[string]struct{}{}
	for _, step := range run.Steps {
		if step.Status == "failed" {
			failedSteps[step.Name] = struct{}{}
			if summary.FailedStep == "" {
				summary.FailedStep = step.Name
			}
		}
		for _, host := range step.Hosts {
			if host.Status == "failed" {
				failedSteps[step.Name] = struct{}{}
				if summary.FailedStep == "" {
					summary.FailedStep = step.Name
				}
				if summary.FailedHost == "" {
					summary.FailedHost = host.Host
				}
			}
		}
	}

	summary.FailedSteps = len(failedSteps)
	if summary.FailedSteps > 0 {
		summary.Status = "failed"
	}

	if run.Status != "" {
		summary.Status = run.Status
	}

	return summary
}
