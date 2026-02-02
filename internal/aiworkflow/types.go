package aiworkflow

import (
	"context"

	"bops/internal/ai"
	"bops/internal/validationenv"
	"bops/internal/validationrun"
)

type Mode string

const (
	ModeGenerate Mode = "generate"
	ModeFix      Mode = "fix"
)

type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

type RiskRule struct {
	Level  RiskLevel
	Reason string
	Regex  string
	Allow  bool
}

type State struct {
	Mode              Mode
	Prompt            string
	Context           map[string]any
	ContextText       string
	SystemPrompt      string
	AgentID           string
	AgentName         string
	AgentRole         string
	BaseYAML          string
	YAML              string
	Questions         []string
	Intent            *Intent
	Issues            []string
	RiskLevel         RiskLevel
	RiskNotes         []string
	RetryCount        int
	MaxRetries        int
	Thought           string
	SkipExecute       bool
	ExecutionSkipped  bool
	ExecutionResult   *validationrun.Result
	ValidationEnv     *validationenv.ValidationEnv
	IsSuccess         bool
	LastError         string
	Summary           string
	NeedsReview       bool
	History           []string
	EventSink         EventSink
	StreamSink        StreamSink
	LoopMetrics       *LoopMetrics
	Plan              []PlanStep
	SubagentSummaries []AgentSummary
}

type Config struct {
	Client       ai.Client
	SystemPrompt string
	MaxRetries   int
	RiskRules    []RiskRule
}

type RunOptions struct {
	SystemPrompt         string
	ContextText          string
	ValidationEnv        *validationenv.ValidationEnv
	SkipExecute          bool
	MaxRetries           int
	EventSink            EventSink
	StreamSink           StreamSink
	BaseYAML             string
	AgentSpec            AgentSpec
	SessionKey           string
	ToolExecutor         ToolExecutor
	ToolNames            []string
	LoopMaxIters         int
	FallbackToPipeline   bool
	FallbackSystemPrompt string
}

type AgentSpec struct {
	Name   string
	Role   string
	Skills []string
}

type AgentSummary struct {
	AgentName string `json:"agent_name"`
	Summary   string `json:"summary"`
}

type PlanStep struct {
	StepName     string   `json:"step_name"`
	Description  string   `json:"description,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

type LoopMetrics struct {
	LoopID       string
	Iterations   int
	ToolCalls    int
	ToolFailures int
	DurationMs   int64
}

type Event struct {
	Node                string         `json:"node"`
	Status              string         `json:"status"`
	Message             string         `json:"message,omitempty"`
	CallID              string         `json:"call_id,omitempty"`
	DisplayName         string         `json:"display_name,omitempty"`
	Stage               string         `json:"stage,omitempty"`
	AgentID             string         `json:"agent_id,omitempty"`
	AgentName           string         `json:"agent_name,omitempty"`
	AgentRole           string         `json:"agent_role,omitempty"`
	LoopID              string         `json:"loop_id,omitempty"`
	Iteration           int            `json:"iteration,omitempty"`
	AgentStatus         string         `json:"agent_status,omitempty"`
	StreamPluginRunning string         `json:"stream_plugin_running,omitempty"`
	Data                map[string]any `json:"data,omitempty"`
}

type EventSink func(Event)

type StreamSink func(ai.StreamDelta)

type ToolExecutor func(ctx context.Context, name string, args map[string]any) (string, error)
