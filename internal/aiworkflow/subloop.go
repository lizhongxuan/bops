package aiworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bops/internal/ai"
)

type SubLoopResult struct {
	YAMLFragment string
	Issues       []string
	Rounds       int
}

func (p *Pipeline) runSubLoop(ctx context.Context, step PlanStep, state *State, opts RunOptions) (SubLoopResult, error) {
	if p == nil || p.cfg.Client == nil {
		return SubLoopResult{}, fmt.Errorf("ai client is not configured")
	}
	maxRounds := 2
	issues := []string{}
	fragment := ""

	for round := 1; round <= maxRounds; round++ {
		emitSubloopEvent(state, step, round, "start", "subloop round start", nil)
		prompt := buildSubloopPrompt(step, issues)
		messages := []ai.Message{
			{Role: "system", Content: pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt)},
			{Role: "user", Content: prompt},
		}
		reply, _, err := p.chatWithThought(ctx, messages, nil)
		if err != nil {
			issues = append(issues, err.Error())
			continue
		}
		nextFragment, err := parseSubloopJSON(reply)
		if err != nil {
			issues = append(issues, err.Error())
			continue
		}
		if err := validateSubPlan(nextFragment, step); err != nil {
			issues = append(issues, err.Error())
			continue
		}
		fragment = nextFragment
		reviewIssues, reviewErr := p.reviewFragment(ctx, fragment, issues, opts)
		if reviewErr != nil {
			issues = append(issues, reviewErr.Error())
			continue
		}
		issues = reviewIssues
		emitSubloopEvent(state, step, round, "done", fmt.Sprintf("issues=%d", len(reviewIssues)), map[string]any{
			"issues": reviewIssues,
		})
		if len(reviewIssues) == 0 {
			return SubLoopResult{YAMLFragment: fragment, Issues: nil, Rounds: round}, nil
		}
	}
	if fragment == "" {
		return SubLoopResult{YAMLFragment: "", Issues: issues, Rounds: maxRounds}, fmt.Errorf("subloop failed")
	}
	return SubLoopResult{YAMLFragment: fragment, Issues: issues, Rounds: maxRounds}, nil
}

func emitSubloopEvent(state *State, step PlanStep, round int, status string, message string, data map[string]any) {
	if state == nil || state.EventSink == nil {
		return
	}
	if data == nil {
		data = map[string]any{}
	}
	data["round"] = round
	data["step_id"] = step.ID
	data["step_name"] = step.StepName
	state.EventSink(Event{
		Node:         "subloop_round",
		Status:       status,
		Message:      message,
		CallID:       fmt.Sprintf("subloop-%s-%d", step.ID, round),
		DisplayName:  "subloop_round",
		Stage:        status,
		AgentID:      state.AgentID,
		AgentName:    state.AgentName,
		AgentRole:    state.AgentRole,
		EventType:    "subloop_round",
		ParentStepID: step.ID,
		Data:         data,
	})
}

func (p *Pipeline) reviewFragment(ctx context.Context, fragment string, knownIssues []string, opts RunOptions) ([]string, error) {
	if p == nil || p.cfg.Client == nil {
		return nil, fmt.Errorf("ai client is not configured")
	}
	prompt := buildReviewPrompt(fragment, knownIssues)
	messages := []ai.Message{
		{Role: "system", Content: pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt)},
		{Role: "user", Content: prompt},
	}
	reply, _, err := p.chatWithThought(ctx, messages, nil)
	if err != nil {
		return nil, err
	}
	issues, err := parseReviewJSON(reply)
	if err != nil {
		return nil, err
	}
	return normalizeQuestions(issues), nil
}

func parseSubloopJSON(reply string) (string, error) {
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		return "", fmt.Errorf("subloop response is not json")
	}
	var payload struct {
		YAMLFragment string `json:"yaml_fragment"`
	}
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.YAMLFragment) == "" {
		return "", fmt.Errorf("yaml_fragment is empty")
	}
	return strings.TrimSpace(payload.YAMLFragment), nil
}
