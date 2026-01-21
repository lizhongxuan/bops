package aiworkflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bops/internal/ai"
	"bops/internal/validationrun"
	"bops/internal/workflow"
)

func (p *Pipeline) inputNormalize(_ context.Context, state *State) (*State, error) {
	if state == nil {
		return nil, errors.New("state is required")
	}
	emitEvent(state, "normalize", "start", "")
	state.Prompt = strings.TrimSpace(state.Prompt)
	state.ContextText = strings.TrimSpace(state.ContextText)
	if state.MaxRetries <= 0 {
		state.MaxRetries = p.cfg.MaxRetries
	}
	if state.SystemPrompt == "" {
		state.SystemPrompt = p.cfg.SystemPrompt
	}
	if state.Mode == "" {
		state.Mode = ModeGenerate
	}
	if state.Mode == ModeGenerate && state.Prompt == "" {
		err := errors.New("prompt is required")
		emitEvent(state, "normalize", "error", err.Error())
		return state, err
	}
	if state.Mode == ModeFix && strings.TrimSpace(state.YAML) == "" {
		err := errors.New("yaml is required")
		emitEvent(state, "normalize", "error", err.Error())
		return state, err
	}
	emitEvent(state, "normalize", "done", "")
	return state, nil
}

func (p *Pipeline) generate(ctx context.Context, state *State) (*State, error) {
	if state.Mode != ModeGenerate {
		return state, nil
	}
	emitEvent(state, "generator", "start", "")
	if p.cfg.Client == nil {
		err := errors.New("ai client is not configured")
		emitEvent(state, "generator", "error", err.Error())
		return state, err
	}
	prompt := buildGeneratePrompt(state.Prompt, state.ContextText)
	messages := []ai.Message{
		{Role: "system", Content: state.SystemPrompt},
		{Role: "user", Content: prompt},
	}
	reply, err := p.cfg.Client.Chat(ctx, messages)
	if err != nil {
		emitEvent(state, "generator", "error", err.Error())
		return state, err
	}
	yamlText, questions, err := extractWorkflowYAML(reply)
	if err != nil {
		emitEvent(state, "generator", "error", err.Error())
		return state, err
	}
	state.YAML = yamlText
	state.Questions = mergeQuestions(state.Questions, questions)
	emitEvent(state, "generator", "done", "")
	return state, nil
}

func (p *Pipeline) fix(ctx context.Context, state *State) (*State, error) {
	emitEvent(state, "fixer", "start", "")
	if p.cfg.Client == nil {
		err := errors.New("ai client is not configured")
		emitEvent(state, "fixer", "error", err.Error())
		return state, err
	}
	if len(state.Issues) == 0 && state.Mode == ModeFix {
		emitEvent(state, "fixer", "skipped", "no issues provided")
		return state, nil
	}
	if state.RetryCount >= state.MaxRetries {
		emitEvent(state, "fixer", "skipped", "max retries reached")
		return state, nil
	}
	prompt := buildFixPrompt(state.YAML, state.Issues, state.LastError)
	messages := []ai.Message{
		{Role: "system", Content: state.SystemPrompt},
		{Role: "user", Content: prompt},
	}
	reply, err := p.cfg.Client.Chat(ctx, messages)
	if err != nil {
		emitEvent(state, "fixer", "error", err.Error())
		return state, err
	}
	yamlText, questions, err := extractWorkflowYAML(reply)
	if err != nil {
		emitEvent(state, "fixer", "error", err.Error())
		return state, err
	}
	if strings.TrimSpace(yamlText) != "" {
		state.History = append(state.History, state.YAML)
		state.YAML = yamlText
	}
	state.Questions = mergeQuestions(state.Questions, questions)
	state.RetryCount++
	emitEvent(state, "fixer", "done", "")
	return state, nil
}

func (p *Pipeline) validate(_ context.Context, state *State) (*State, error) {
	emitEvent(state, "validator", "start", "")
	trimmed := strings.TrimSpace(state.YAML)
	if trimmed == "" {
		state.Issues = []string{"yaml is empty"}
		state.IsSuccess = false
		emitEvent(state, "validator", "error", "yaml is empty")
		return state, nil
	}
	wf, err := workflow.Load([]byte(trimmed))
	if err != nil {
		state.Issues = []string{err.Error()}
		state.IsSuccess = false
		emitEvent(state, "validator", "error", err.Error())
		return state, nil
	}
	var issues []string
	if err := wf.Validate(); err != nil {
		if vErr, ok := err.(*workflow.ValidationError); ok {
			issues = append(issues, vErr.Issues...)
		} else {
			issues = append(issues, err.Error())
		}
	}
	issues = append(issues, guardrailIssues(wf, trimmed)...)
	if len(issues) > 0 {
		state.Issues = dedupeStrings(issues)
		state.IsSuccess = false
		emitEvent(state, "validator", "error", "validation failed")
		return state, nil
	}
	state.Issues = nil
	emitEvent(state, "validator", "done", "")
	return state, nil
}

func (p *Pipeline) safetyCheck(_ context.Context, state *State) (*State, error) {
	emitEvent(state, "safety", "start", "")
	level, notes := EvaluateRisk(state.YAML, p.cfg.RiskRules)
	state.RiskLevel = level
	state.RiskNotes = notes
	if state.RiskLevel == RiskLevelHigh {
		state.SkipExecute = true
		if updated, err := forceManualApprove(state.YAML); err == nil && strings.TrimSpace(updated) != "" {
			state.YAML = updated
		}
	}
	emitEvent(state, "safety", "done", fmt.Sprintf("risk=%s", state.RiskLevel))
	return state, nil
}

func (p *Pipeline) execute(ctx context.Context, state *State) (*State, error) {
	if state.SkipExecute || state.ValidationEnv == nil {
		state.ExecutionSkipped = true
		state.IsSuccess = true
		emitEvent(state, "executor", "skipped", "execution skipped")
		return state, nil
	}
	emitEvent(state, "executor", "start", "")
	result, err := validationrun.Runner(ctx, *state.ValidationEnv, state.YAML)
	state.ExecutionResult = &result
	if err != nil || result.Status != "success" {
		state.IsSuccess = false
		if err != nil {
			state.LastError = err.Error()
		} else {
			state.LastError = strings.TrimSpace(result.Stderr)
		}
		emitEvent(state, "executor", "error", state.LastError)
		return state, nil
	}
	state.IsSuccess = true
	emitEvent(state, "executor", "done", "")
	return state, nil
}

func (p *Pipeline) summarize(_ context.Context, state *State) (*State, error) {
	emitEvent(state, "summarizer", "start", "")
	steps := countSteps(state.YAML)
	issues := len(state.Issues)
	state.Summary = fmt.Sprintf("steps=%d risk=%s issues=%d", steps, state.RiskLevel, issues)
	emitEvent(state, "summarizer", "done", state.Summary)
	return state, nil
}

func (p *Pipeline) humanGate(_ context.Context, state *State) (*State, error) {
	emitEvent(state, "human_gate", "start", "")
	if state.RiskLevel != RiskLevelLow || len(state.Issues) > 0 || !state.IsSuccess {
		state.NeedsReview = true
	}
	emitEvent(state, "human_gate", "done", "")
	return state, nil
}

func emitEvent(state *State, node, status, message string) {
	if state == nil || state.EventSink == nil {
		return
	}
	state.EventSink(Event{
		Node:    node,
		Status:  status,
		Message: message,
	})
}

func countSteps(yamlText string) int {
	lines := strings.Split(yamlText, "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- name:") {
			count++
		}
	}
	return count
}
