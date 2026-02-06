package report

import "bops/runner/state"

type FailureReport struct {
	RunID  string         `json:"run_id"`
	Step   string         `json:"step"`
	Host   string         `json:"host"`
	Error  string         `json:"error"`
	Output map[string]any `json:"output,omitempty"`
}

func FailureDetails(run state.RunState) FailureReport {
	report := FailureReport{RunID: run.RunID}

	for _, step := range run.Steps {
		if step.Status == "failed" && report.Step == "" {
			report.Step = step.Name
		}
		for _, host := range step.Hosts {
			if host.Status == "failed" {
				if report.Step == "" {
					report.Step = step.Name
				}
				if report.Host == "" {
					report.Host = host.Host
					report.Error = host.Message
					report.Output = host.Output
					return report
				}
			}
		}
	}

	return report
}
