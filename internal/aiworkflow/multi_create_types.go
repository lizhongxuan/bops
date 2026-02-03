package aiworkflow

import "time"

// StepPatch represents a single step update produced by Coder/Reviewer.
// It only contains step-level fields; top-level workflow fields are not allowed.
type StepPatch struct {
	StepID   string         `json:"step_id,omitempty"`
	StepName string         `json:"step_name"`
	Action   string         `json:"action"`
	Targets  []string       `json:"targets,omitempty"`
	With     map[string]any `json:"with,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Source   string         `json:"source,omitempty"` // "coder" | "reviewer"
}

// ReviewTask is a queued unit for reviewer validation.
type ReviewTask struct {
	StepID         string    `json:"step_id"`
	Patch          StepPatch `json:"patch"`
	Attempt        int       `json:"attempt"`
	ValidationHost string    `json:"validation_host,omitempty"`
	Status         string    `json:"status,omitempty"` // pending/running/done/failed
	Error          string    `json:"error,omitempty"`
}

// ReviewResult captures reviewer output for a step.
type ReviewResult struct {
	StepID   string     `json:"step_id"`
	Status   StepStatus `json:"status"`
	Summary  string     `json:"summary,omitempty"`
	Issues   []string   `json:"issues,omitempty"`
	Attempts int        `json:"attempts,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// DraftState holds per-draft progress for multi-agent creation.
type DraftState struct {
	DraftID   string                  `json:"draft_id"`
	Plan      []PlanStep              `json:"plan,omitempty"`
	Steps     map[string]StepPatch    `json:"steps_map,omitempty"`
	Reviews   map[string]ReviewResult `json:"review_results,omitempty"`
	Metrics   map[string]int          `json:"metrics,omitempty"`
	BaseYAML  string                  `json:"-"`
	UpdatedAt time.Time               `json:"updated_at"`
}
