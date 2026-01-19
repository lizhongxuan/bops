package core

import "time"

type EventType string

type EventLevel string

const (
	EventWorkflowStart EventType = "workflow_start"
	EventWorkflowEnd   EventType = "workflow_end"
	EventStepStart     EventType = "step_start"
	EventStepEnd       EventType = "step_end"
	EventStepFailed    EventType = "step_failed"
	EventPlanGenerated EventType = "plan_generated"
	EventAgentOutput   EventType = "agent_output"
)

const (
	EventInfo  EventLevel = "info"
	EventWarn  EventLevel = "warn"
	EventError EventLevel = "error"
)

type Event struct {
	ID         string         `json:"id"`
	Type       EventType      `json:"type"`
	Level      EventLevel     `json:"level"`
	Time       time.Time      `json:"time"`
	WorkflowID string         `json:"workflow_id,omitempty"`
	RunID      string         `json:"run_id,omitempty"`
	Step       string         `json:"step,omitempty"`
	Host       string         `json:"host,omitempty"`
	Message    string         `json:"message,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}
