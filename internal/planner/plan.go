package planner

import "time"

type Plan struct {
	ID           string     `json:"id"`
	WorkflowName string     `json:"workflow_name"`
	CreatedAt    time.Time  `json:"created_at"`
	Steps        []StepPlan `json:"steps"`
}

type StepPlan struct {
	Name    string           `json:"name"`
	Action  string           `json:"action"`
	Targets []string         `json:"targets"`
	Changes []ResourceChange `json:"changes"`
}

type ResourceChange struct {
	ResourceID string               `json:"resource_id"`
	Diff       map[string]DiffEntry `json:"diff"`
}

type DiffEntry struct {
	Current any `json:"current"`
	Desired any `json:"desired"`
}
