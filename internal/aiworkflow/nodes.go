package aiworkflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bops/internal/ai"
	"bops/internal/logging"
	"bops/internal/validationrun"
	"bops/internal/workflow"
	"go.uber.org/zap"
)

func (p *Pipeline) inputNormalize(_ context.Context, state *State) (*State, error) {
	if state == nil {
		return nil, errors.New("state is required")
	}
	logging.L().Debug("aiworkflow normalize start")
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
	if state.Mode == ModeGenerate && isGreetingPrompt(state.Prompt) {
		state.Intent = &Intent{Missing: []string{"goal"}}
	}
	emitEvent(state, "normalize", "done", "")
	logging.L().Debug("aiworkflow normalize done",
		zap.String("mode", string(state.Mode)),
		zap.Int("prompt_len", len(state.Prompt)),
		zap.Int("yaml_len", len(state.YAML)),
	)
	return state, nil
}

func (p *Pipeline) generate(ctx context.Context, state *State) (*State, error) {
	if state.Mode != ModeGenerate {
		return state, nil
	}
	logging.L().Debug("aiworkflow generate start")
	emitEvent(state, "generator", "start", "")
	if p.cfg.Client == nil {
		err := errors.New("ai client is not configured")
		emitEvent(state, "generator", "error", err.Error())
		return state, err
	}
	prompt := buildGeneratePrompt(state.Prompt, state.ContextText, state.BaseYAML)
	messages := []ai.Message{
		{Role: "system", Content: state.SystemPrompt},
		{Role: "user", Content: prompt},
	}
	reply, thought, err := p.chatWithThought(ctx, messages, state.StreamSink)
	if err != nil {
		emitEvent(state, "generator", "error", err.Error())
		return state, err
	}
	state.Thought = strings.TrimSpace(thought)
	if state.Thought != "" {
		logging.L().Debug("aiworkflow generate thought captured", zap.Int("thought_len", len(state.Thought)))
	}
	yamlText, questions, err := extractWorkflowYAML(reply)
	if err != nil {
		emitEvent(state, "generator", "error", err.Error())
		return state, err
	}
	if strings.TrimSpace(state.BaseYAML) != "" {
		yamlText = mergeStepsIntoBase(state.BaseYAML, yamlText)
	}
	state.YAML = yamlText
	state.Questions = mergeQuestions(state.Questions, questions)
	if strings.TrimSpace(state.YAML) != "" {
		emitEventWithData(state, "generator", "done", "", map[string]any{"yaml": state.YAML})
	} else {
		emitEvent(state, "generator", "done", "")
	}
	logging.L().Debug("aiworkflow generate done",
		zap.Int("yaml_len", len(state.YAML)),
		zap.Int("questions", len(state.Questions)),
	)
	return state, nil
}

func (p *Pipeline) fix(ctx context.Context, state *State) (*State, error) {
	logging.L().Debug("aiworkflow fix start", zap.Int("issues", len(state.Issues)))
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
	reply, thought, err := p.chatWithThought(ctx, messages, state.StreamSink)
	if err != nil {
		emitEvent(state, "fixer", "error", err.Error())
		return state, err
	}
	state.Thought = strings.TrimSpace(thought)
	if state.Thought != "" {
		logging.L().Debug("aiworkflow fix thought captured", zap.Int("thought_len", len(state.Thought)))
	}
	yamlText, questions, err := extractWorkflowYAML(reply)
	if err != nil {
		emitEvent(state, "fixer", "error", err.Error())
		return state, err
	}
	if strings.TrimSpace(state.BaseYAML) != "" {
		yamlText = mergeStepsIntoBase(state.BaseYAML, yamlText)
	}
	prevYAML := state.YAML
	if strings.TrimSpace(yamlText) != "" {
		state.History = append(state.History, state.YAML)
		state.YAML = yamlText
	}
	state.Questions = mergeQuestions(state.Questions, questions)
	state.RetryCount++
	if strings.TrimSpace(state.YAML) != "" {
		emitEventWithData(state, "fixer", "done", "", map[string]any{
			"yaml":      state.YAML,
			"prev_yaml": prevYAML,
		})
	} else {
		emitEvent(state, "fixer", "done", "")
	}
	logging.L().Debug("aiworkflow fix done",
		zap.Int("yaml_len", len(state.YAML)),
		zap.Int("questions", len(state.Questions)),
		zap.Int("retries", state.RetryCount),
	)
	return state, nil
}

func (p *Pipeline) validate(_ context.Context, state *State) (*State, error) {
	logging.L().Debug("aiworkflow validate start", zap.Int("yaml_len", len(state.YAML)))
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
	logging.L().Debug("aiworkflow validate done")
	return state, nil
}

func (p *Pipeline) safetyCheck(_ context.Context, state *State) (*State, error) {
	logging.L().Debug("aiworkflow safety start")
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
	logging.L().Debug("aiworkflow safety done", zap.String("risk_level", string(state.RiskLevel)))
	return state, nil
}

func (p *Pipeline) execute(ctx context.Context, state *State) (*State, error) {
	if state.SkipExecute || state.ValidationEnv == nil {
		state.ExecutionSkipped = true
		state.IsSuccess = true
		emitEvent(state, "executor", "skipped", "execution skipped")
		return state, nil
	}
	logging.L().Debug("aiworkflow execute start",
		zap.String("env", state.ValidationEnv.Name),
		zap.String("env_type", string(state.ValidationEnv.Type)),
	)
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
	logging.L().Debug("aiworkflow execute done", zap.String("status", result.Status))
	return state, nil
}

func (p *Pipeline) summarize(_ context.Context, state *State) (*State, error) {
	logging.L().Debug("aiworkflow summarize start")
	steps := countSteps(state.YAML)
	issues := len(state.Issues)
	state.Summary = fmt.Sprintf("steps=%d risk=%s issues=%d", steps, state.RiskLevel, issues)
	logging.L().Debug("aiworkflow summarize done", zap.String("summary", state.Summary))
	return state, nil
}

func (p *Pipeline) humanGate(_ context.Context, state *State) (*State, error) {
	logging.L().Debug("aiworkflow human gate start")
	if state.RiskLevel != RiskLevelLow || len(state.Issues) > 0 || !state.IsSuccess {
		state.NeedsReview = true
	}
	logging.L().Debug("aiworkflow human gate done", zap.Bool("needs_review", state.NeedsReview))
	return state, nil
}

func emitEvent(state *State, node, status, message string) {
	emitEventWithData(state, node, status, message, nil)
}

func emitEventWithData(state *State, node, status, message string, data map[string]any) {
	if state == nil || state.EventSink == nil {
		return
	}
	displayName := mapNodeDisplayName(node)
	streamPluginRunning := ""
	if data != nil {
		if value, ok := data["stream_plugin_running"].(string); ok {
			streamPluginRunning = value
		}
	}
	state.EventSink(Event{
		Node:                node,
		Status:              status,
		Message:             message,
		CallID:              node,
		DisplayName:         displayName,
		Stage:               status,
		AgentID:             state.AgentID,
		AgentName:           state.AgentName,
		AgentRole:           state.AgentRole,
		StreamPluginRunning: streamPluginRunning,
		Data:                data,
	})
}

func mapNodeDisplayName(node string) string {
	switch node {
	case "normalize":
		return "规范化输入"
	case "intent_extract":
		return "意图解析"
	case "planner":
		return "生成计划"
	case "question_gate":
		return "问题补全"
	case "generator":
		return "生成工作流"
	case "validator":
		return "校验工作流"
	case "safety":
		return "安全检查"
	case "executor":
		return "执行验证"
	case "fixer":
		return "修复工作流"
	case "summarizer":
		return "总结结果"
	case "human_gate":
		return "人工确认"
	default:
		return node
	}
}

func (p *Pipeline) chatWithThought(ctx context.Context, messages []ai.Message, sink StreamSink) (string, string, error) {
	if p.cfg.Client == nil {
		return "", "", errors.New("ai client is not configured")
	}
	started := time.Now()
	logging.L().Info("llm prompt",
		zap.Int("message_count", len(messages)),
		zap.Any("messages", messages),
	)
	logResponse := func(reply, thought string, err error) {
		if err != nil {
			logging.L().Error("llm response error",
				zap.Error(err),
				zap.Duration("elapsed", time.Since(started)),
			)
			return
		}
		logging.L().Info("llm response",
			zap.Int("reply_len", len(reply)),
			zap.Int("thought_len", len(thought)),
			zap.Duration("elapsed", time.Since(started)),
		)
	}
	reply, thought, err := runChatWithADK(ctx, p.cfg.Client, messages, sink)
	logResponse(reply, thought, err)
	return reply, thought, err
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
