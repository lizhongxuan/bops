package aiworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"bops/internal/ai"
)

type planPayload struct {
	Steps []PlanStep `json:"steps"`
}

func (p *Pipeline) plan(ctx context.Context, state *State) (*State, error) {
	if state.Mode != ModeGenerate {
		emitEvent(state, "planner", "skipped", "mode is not generate")
		return state, nil
	}
	if state.Intent != nil && len(state.Intent.Missing) > 0 {
		emitEvent(state, "planner", "skipped", "intent missing data")
		return state, nil
	}
	if p.cfg.Client == nil {
		err := errors.New("ai client is not configured")
		emitEvent(state, "planner", "error", err.Error())
		return state, err
	}
	emitEvent(state, "planner", "start", "")
	intentType := IntentExplain
	if state.Intent != nil && state.Intent.Type != "" {
		intentType = state.Intent.Type
	}
	prompt := buildPlanPrompt(state.Prompt, intentType, state.ContextText)
	messages := []ai.Message{
		{Role: "system", Content: state.SystemPrompt},
		{Role: "user", Content: prompt},
	}
	reply, _, err := p.chatWithThought(ctx, messages, state.StreamSink)
	if err != nil {
		emitEvent(state, "planner", "warn", err.Error())
		return state, nil
	}
	steps, err := parsePlanJSON(reply)
	if err != nil {
		emitEvent(state, "planner", "warn", err.Error())
		return state, nil
	}
	state.Plan = steps
	if len(steps) > 0 {
		emitEventWithData(state, "planner", "done", "", map[string]any{"plan": steps})
	} else {
		emitEvent(state, "planner", "done", "")
	}
	return state, nil
}

func buildPlanPrompt(prompt string, intentType IntentType, contextText string) string {
	builder := strings.Builder{}
	if contextText != "" {
		builder.WriteString("Context:\n")
		builder.WriteString(contextText)
		builder.WriteString("\n\n")
	}
	builder.WriteString("User request:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\n")
	builder.WriteString("Intent type: ")
	builder.WriteString(string(intentType))
	builder.WriteString("\n\n")
	builder.WriteString("Return JSON only. Output should be a list of steps with keys: step_name, description, dependencies. ")
	builder.WriteString("dependencies is an array of step_name. Do not include markdown or extra text.")
	return builder.String()
}

func parsePlanJSON(reply string) ([]PlanStep, error) {
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		return nil, errors.New("plan response is not json")
	}
	var steps []PlanStep
	if err := json.Unmarshal([]byte(jsonText), &steps); err == nil {
		return normalizePlanSteps(steps), nil
	}
	var payload planPayload
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, err
	}
	return normalizePlanSteps(payload.Steps), nil
}

func normalizePlanSteps(steps []PlanStep) []PlanStep {
	out := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		name := strings.TrimSpace(step.StepName)
		if name == "" {
			continue
		}
		next := PlanStep{
			StepName:    name,
			Description: strings.TrimSpace(step.Description),
		}
		for _, dep := range step.Dependencies {
			trimmed := strings.TrimSpace(dep)
			if trimmed == "" {
				continue
			}
			next.Dependencies = append(next.Dependencies, trimmed)
		}
		out = append(out, next)
	}
	return out
}
