package aiworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bops/internal/ai"
	"bops/internal/logging"
	"go.uber.org/zap"
)

type loopAction struct {
	Action    string         `json:"action"`
	Tool      string         `json:"tool,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Result    string         `json:"result,omitempty"`
	YAML      string         `json:"yaml,omitempty"`
	Questions []string       `json:"questions,omitempty"`
	Message   string         `json:"message,omitempty"`
}

func (p *Pipeline) RunAgentLoop(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	if p == nil || p.cfg.Client == nil {
		return nil, fmt.Errorf("ai client is not configured")
	}
	logging.L().Debug("aiworkflow loop invoke",
		zap.Int("prompt_len", len(prompt)),
		zap.Int("context_items", len(context)),
	)
	spec := normalizeAgentSpec(opts.AgentSpec)
	opts.EventSink = wrapEventSinkWithAgent(opts.EventSink, spec)
	state := &State{
		Mode:         ModeGenerate,
		Prompt:       prompt,
		Context:      context,
		ContextText:  opts.ContextText,
		SystemPrompt: pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt),
		AgentID:      spec.Name,
		AgentName:    spec.Name,
		AgentRole:    spec.Role,
		MaxRetries:   pickMaxRetries(opts.MaxRetries, p.cfg.MaxRetries),
		EventSink:    opts.EventSink,
		StreamSink:   opts.StreamSink,
		BaseYAML:     opts.BaseYAML,
	}
	loopState, err := p.runAgentLoop(ctx, state, opts)
	if err != nil && opts.FallbackToPipeline {
		logging.L().Warn("agent loop fallback to pipeline", zap.Error(err))
		fallbackOpts := opts
		fallbackOpts.ToolExecutor = nil
		fallbackOpts.ToolNames = nil
		fallbackOpts.LoopMaxIters = 0
		fallbackOpts.FallbackToPipeline = false
		fallbackOpts.SystemPrompt = pickSystemPrompt(opts.FallbackSystemPrompt, p.cfg.SystemPrompt)
		fallbackState, fallbackErr := p.RunGenerate(ctx, prompt, context, fallbackOpts)
		if fallbackErr == nil {
			if loopState != nil && loopState.LoopMetrics != nil {
				fallbackState.LoopMetrics = loopState.LoopMetrics
			}
			return fallbackState, nil
		}
		return loopState, fallbackErr
	}
	return loopState, err
}

func (p *Pipeline) runAgentLoop(ctx context.Context, state *State, opts RunOptions) (*State, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	maxIters := opts.LoopMaxIters
	if maxIters <= 0 {
		maxIters = 6
	}
	loopID := fmt.Sprintf("loop-%d", time.Now().UnixNano())
	toolNames := opts.ToolNames
	toolHistory := make([]string, 0, maxIters)
	consecutiveFailures := 0
	const maxPromptChars = 14000
	started := time.Now()
	lastIteration := 0
	toolCalls := 0
	toolFailures := 0
	defer func() {
		if state == nil {
			return
		}
		state.LoopMetrics = &LoopMetrics{
			LoopID:       loopID,
			Iterations:   lastIteration,
			ToolCalls:    toolCalls,
			ToolFailures: toolFailures,
			DurationMs:   time.Since(started).Milliseconds(),
		}
	}()

	for iteration := 1; iteration <= maxIters; iteration++ {
		lastIteration = iteration
		if err := ctx.Err(); err != nil {
			return state, err
		}
		promptText := buildLoopPrompt(state.Prompt, state.ContextText, state.BaseYAML, toolNames, toolHistory, iteration)
		if len(promptText) > maxPromptChars {
			return state, fmt.Errorf("loop prompt too long")
		}
		messages := []ai.Message{
			{Role: "system", Content: state.SystemPrompt},
			{Role: "user", Content: promptText},
		}

		reply, thought, err := p.chatWithThought(ai.WithModelRole(ctx, ai.RolePlanner), messages, state.StreamSink)
		if err != nil {
			consecutiveFailures++
			if consecutiveFailures >= 2 {
				return state, err
			}
			continue
		}
		state.Thought = strings.TrimSpace(thought)
		action, err := parseLoopAction(reply)
		if err != nil {
			consecutiveFailures++
			if consecutiveFailures >= 2 {
				return state, err
			}
			continue
		}

		switch normalizeLoopAction(action.Action) {
		case "final":
			yamlText := strings.TrimSpace(action.YAML)
			if yamlText != "" {
				state.YAML = normalizeWorkflowYAML(yamlText)
				state.Questions = action.Questions
				return state, nil
			}
			if action.Result != "" {
				yamlText, questions, err := extractWorkflowYAML(action.Result)
				if err == nil {
					state.YAML = yamlText
					state.Questions = mergeQuestions(state.Questions, questions)
					return state, nil
				}
			}
			yamlText, questions, err := extractWorkflowYAML(reply)
			if err == nil {
				state.YAML = yamlText
				state.Questions = mergeQuestions(state.Questions, questions)
				return state, nil
			}
			return state, fmt.Errorf("loop final output missing yaml")
		case "need_more_info":
			if len(action.Questions) > 0 {
				state.Questions = mergeQuestions(state.Questions, action.Questions)
			}
			return state, nil
		case "tool_call":
			toolName := strings.TrimSpace(action.Tool)
			if toolName == "" {
				return state, fmt.Errorf("tool_call missing tool name")
			}
			if opts.ToolExecutor == nil {
				return state, fmt.Errorf("tool executor is not configured")
			}
			toolCalls++
			callID := fmt.Sprintf("%s-%d-%s", loopID, iteration, toolName)
			emitLoopEvent(state, loopEventPayload{
				LoopID:      loopID,
				Iteration:   iteration,
				AgentStatus: "tool_call",
				Node:        toolName,
				Status:      "start",
				Message:     fmt.Sprintf("调用工具 %s", toolName),
				CallID:      callID,
				DisplayName: fmt.Sprintf("使用工具 %s", toolName),
			})
			output, err := opts.ToolExecutor(ctx, toolName, action.Args)
			if err != nil {
				toolFailures++
				emitLoopEvent(state, loopEventPayload{
					LoopID:      loopID,
					Iteration:   iteration,
					AgentStatus: "tool_result",
					Node:        toolName,
					Status:      "error",
					Message:     err.Error(),
					CallID:      callID,
					DisplayName: fmt.Sprintf("使用工具 %s", toolName),
					Data: map[string]any{
						"tool_output_content": err.Error(),
					},
				})
				consecutiveFailures++
				if consecutiveFailures >= 2 {
					return state, err
				}
				toolHistory = append(toolHistory, fmt.Sprintf("tool=%s error=%s", toolName, err.Error()))
				continue
			}
			consecutiveFailures = 0
			emitLoopEvent(state, loopEventPayload{
				LoopID:      loopID,
				Iteration:   iteration,
				AgentStatus: "tool_result",
				Node:        toolName,
				Status:      "done",
				Message:     output,
				CallID:      callID,
				DisplayName: fmt.Sprintf("使用工具 %s", toolName),
				Data: map[string]any{
					"tool_output_content": output,
				},
			})
			summary := ai.SummarizeToolOutput(output, 400)
			toolHistory = append(toolHistory, fmt.Sprintf("tool=%s output=%s", toolName, summary))
			continue
		default:
			return state, fmt.Errorf("unknown loop action")
		}
	}
	return state, fmt.Errorf("loop max iterations reached")
}

func parseLoopAction(reply string) (loopAction, error) {
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return loopAction{}, fmt.Errorf("empty loop response")
	}
	if jsonText := extractJSONBlock(trimmed); jsonText != "" {
		trimmed = jsonText
	}
	var action loopAction
	if err := json.Unmarshal([]byte(trimmed), &action); err != nil {
		return loopAction{}, fmt.Errorf("invalid loop json: %w", err)
	}
	return action, nil
}

func normalizeLoopAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "tool", "tool_call", "call_tool":
		return "tool_call"
	case "final", "done", "finish":
		return "final"
	case "need_more_info", "question", "questions":
		return "need_more_info"
	default:
		return ""
	}
}

type loopEventPayload struct {
	LoopID      string
	Iteration   int
	AgentStatus string
	Node        string
	Status      string
	Message     string
	CallID      string
	DisplayName string
	Data        map[string]any
}

func emitLoopEvent(state *State, payload loopEventPayload) {
	if state == nil || state.EventSink == nil {
		return
	}
	state.EventSink(Event{
		Node:        payload.Node,
		Status:      payload.Status,
		Message:     payload.Message,
		CallID:      payload.CallID,
		DisplayName: payload.DisplayName,
		Stage:       payload.Status,
		AgentID:     state.AgentID,
		AgentName:   state.AgentName,
		AgentRole:   state.AgentRole,
		LoopID:      payload.LoopID,
		Iteration:   payload.Iteration,
		AgentStatus: payload.AgentStatus,
		Data:        payload.Data,
	})
}
