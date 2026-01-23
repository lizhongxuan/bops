package aiworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"bops/internal/ai"
)

type Intent struct {
	Goal        string   `json:"goal"`
	Targets     []string `json:"targets"`
	Constraints []string `json:"constraints"`
	Resources   []string `json:"resources"`
	Actions     []string `json:"actions"`
	Missing     []string `json:"missing"`
}

const intentSystemPrompt = "You are a workflow intent extractor. Return JSON only."
const intentExtractTimeout = 8 * time.Second

func (p *Pipeline) intentExtract(ctx context.Context, state *State) (*State, error) {
	if state.Mode != ModeGenerate {
		emitEvent(state, "intent_extract", "skipped", "mode is not generate")
		return state, nil
	}
	emitEvent(state, "intent_extract", "start", "")
	if p.cfg.Client == nil {
		err := errors.New("ai client is not configured")
		emitEvent(state, "intent_extract", "error", err.Error())
		return state, err
	}
	prompt := buildIntentPrompt(state.Prompt, state.ContextText)
	messages := []ai.Message{
		{Role: "system", Content: intentSystemPrompt},
		{Role: "user", Content: prompt},
	}
	state.Intent = nil
	state.Questions = nil
	ctxWithTimeout, cancel := context.WithTimeout(ctx, intentExtractTimeout)
	defer cancel()
	reply, err := p.cfg.Client.Chat(ctxWithTimeout, messages)
	if err != nil {
		emitEvent(state, "intent_extract", "warn", err.Error())
		return state, nil
	}
	intent, err := parseIntentResponse(reply)
	if err != nil {
		emitEvent(state, "intent_extract", "warn", err.Error())
		return state, nil
	}
	intent.Missing = normalizeMissing(intent.Missing)
	state.Intent = intent
	emitEvent(state, "intent_extract", "done", "")
	return state, nil
}

func buildIntentPrompt(prompt, contextText string) string {
	builder := strings.Builder{}
	if contextText != "" {
		builder.WriteString("Context:\n")
		builder.WriteString(contextText)
		builder.WriteString("\n\n")
	}
	builder.WriteString("User request:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\n")
	builder.WriteString("Return JSON only with keys: goal, targets, constraints, resources, actions, missing. ")
	builder.WriteString("missing should list fields needed to build executable steps. Do not include markdown.")
	return builder.String()
}

func parseIntentResponse(reply string) (*Intent, error) {
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		return nil, errors.New("intent response is not json")
	}
	var intent Intent
	if err := json.Unmarshal([]byte(jsonText), &intent); err != nil {
		return nil, err
	}
	return &intent, nil
}
