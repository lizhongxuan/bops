package aiworkflow

import (
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
	Mode             Mode
	Prompt           string
	Context          map[string]any
	ContextText      string
	SystemPrompt     string
	YAML             string
	Issues           []string
	RiskLevel        RiskLevel
	RiskNotes        []string
	RetryCount       int
	MaxRetries       int
	SkipExecute      bool
	ExecutionSkipped bool
	ExecutionResult  *validationrun.Result
	ValidationEnv    *validationenv.ValidationEnv
	IsSuccess        bool
	LastError        string
	Summary          string
	NeedsReview      bool
	History          []string
	EventSink        EventSink
}

type Config struct {
	Client       ai.Client
	SystemPrompt string
	MaxRetries   int
	RiskRules    []RiskRule
}

type RunOptions struct {
	SystemPrompt  string
	ContextText   string
	ValidationEnv *validationenv.ValidationEnv
	SkipExecute   bool
	MaxRetries    int
	EventSink     EventSink
}

type Event struct {
	Node    string `json:"node"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type EventSink func(Event)
