package state

import "time"

type ResourceState struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Desired   map[string]any `json:"desired"`
	Current   map[string]any `json:"current"`
	Diff      map[string]any `json:"diff"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type HostResult struct {
	Host       string         `json:"host"`
	Status     string         `json:"status"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	Message    string         `json:"message"`
	Output     map[string]any `json:"output"`
}

type StepState struct {
	Name       string                `json:"name"`
	Status     string                `json:"status"`
	StartedAt  time.Time             `json:"started_at"`
	FinishedAt time.Time             `json:"finished_at"`
	Hosts      map[string]HostResult `json:"hosts"`
}

type RunState struct {
	RunID           string                   `json:"run_id"`
	WorkflowName    string                   `json:"workflow_name"`
	WorkflowVersion string                   `json:"workflow_version"`
	Status          string                   `json:"status"`
	Message         string                   `json:"message,omitempty"`
	StartedAt       time.Time                `json:"started_at"`
	FinishedAt      time.Time                `json:"finished_at"`
	Steps           []StepState              `json:"steps"`
	Resources       map[string]ResourceState `json:"resources"`
}
