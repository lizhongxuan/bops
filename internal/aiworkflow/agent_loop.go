package aiworkflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"bops/internal/ai"
	"bops/runner/logging"
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
	started := time.Now()
	logging.L().Info("agent loop start",
		zap.String("agent", spec.Name),
		zap.String("role", spec.Role),
		zap.Int("prompt_len", len(prompt)),
	)
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
			logging.L().Info("agent loop end",
				zap.String("agent", spec.Name),
				zap.String("role", spec.Role),
				zap.Bool("fallback", true),
				zap.Duration("elapsed", time.Since(started)),
			)
			return fallbackState, nil
		}
		logging.L().Error("agent loop end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.Bool("fallback", true),
			zap.Error(fallbackErr),
			zap.Duration("elapsed", time.Since(started)),
		)
		return loopState, fallbackErr
	}
	if err != nil {
		logging.L().Error("agent loop end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.Error(err),
			zap.Duration("elapsed", time.Since(started)),
		)
	} else {
		logging.L().Info("agent loop end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.Duration("elapsed", time.Since(started)),
		)
	}
	return loopState, err
}

func (p *Pipeline) RunCoordinatorLoop(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	if p == nil || p.cfg.Client == nil {
		return nil, fmt.Errorf("ai client is not configured")
	}
	spec := normalizeAgentSpec(opts.AgentSpec)
	opts.EventSink = wrapEventSinkWithAgent(opts.EventSink, spec)
	started := time.Now()
	logging.L().Info("coordinator loop start",
		zap.String("agent", spec.Name),
		zap.String("role", spec.Role),
		zap.Int("prompt_len", len(prompt)),
	)
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
		StepStatuses: make(map[string]StepStatus),
	}
	store := NewStateStore(opts.BaseYAML)

	if _, err := p.plan(ctx, state); err != nil {
		logging.L().Error("coordinator loop end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.Error(err),
			zap.Duration("elapsed", time.Since(started)),
		)
		return state, err
	}
	if len(state.Plan) == 0 {
		logging.L().Info("coordinator loop end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.Duration("elapsed", time.Since(started)),
		)
		return state, nil
	}
	store.SetPlan(state.Plan)

	for i := range state.Plan {
		step := &state.Plan[i]
		if strings.TrimSpace(step.ID) == "" {
			step.ID = normalizePlanID(step.StepName, i)
		}
		if step.Status == "" {
			step.Status = StepStatusPending
		}
		setStepStatus(state, step.ID, step.Status)
	}

	for i := range state.Plan {
		step := &state.Plan[i]
		state.CurrentStepID = step.ID
		step.Status = StepStatusInProgress
		setStepStatus(state, step.ID, StepStatusInProgress)
		emitStepEvent(state, "plan_step_start", *step, "start", "step start", nil)

		result, err := p.runSubLoop(ctx, *step, state, opts)
		if err != nil {
			step.Status = StepStatusFailed
			setStepStatus(state, step.ID, StepStatusFailed)
			logging.L().Error("coordinator loop end",
				zap.String("agent", spec.Name),
				zap.String("role", spec.Role),
				zap.Error(err),
				zap.Duration("elapsed", time.Since(started)),
			)
			return state, err
		}
		if strings.TrimSpace(result.YAMLFragment) != "" {
			if err := store.UpdateYAMLFragment(result.YAMLFragment, step.ID); err != nil {
				step.Status = StepStatusFailed
				setStepStatus(state, step.ID, StepStatusFailed)
				logging.L().Error("coordinator loop end",
					zap.String("agent", spec.Name),
					zap.String("role", spec.Role),
					zap.Error(err),
					zap.Duration("elapsed", time.Since(started)),
				)
				return state, err
			}
			emitStepEvent(state, "merge_patch", *step, "done", "merge patch", map[string]any{
				"yaml_fragment": result.YAMLFragment,
			})
			snapshot := store.Snapshot()
			state.YAML = snapshot.YAML
			state.History = snapshot.History
		}

		step.Status = StepStatusDone
		setStepStatus(state, step.ID, StepStatusDone)
		emitStepEvent(state, "plan_step_done", *step, "done", "step done", nil)
	}
	logging.L().Info("coordinator loop end",
		zap.String("agent", spec.Name),
		zap.String("role", spec.Role),
		zap.Duration("elapsed", time.Since(started)),
	)
	return state, nil
}

func emitStepEvent(state *State, eventType string, step PlanStep, status string, message string, data map[string]any) {
	if state == nil || state.EventSink == nil {
		return
	}
	if data == nil {
		data = map[string]any{}
	}
	if _, ok := data["step_id"]; !ok {
		data["step_id"] = step.ID
	}
	if _, ok := data["step_name"]; !ok {
		data["step_name"] = step.StepName
	}
	state.EventSink(Event{
		Node:         eventType,
		Status:       status,
		Message:      message,
		CallID:       eventType,
		DisplayName:  eventType,
		Stage:        status,
		AgentID:      state.AgentID,
		AgentName:    state.AgentName,
		AgentRole:    state.AgentRole,
		EventType:    eventType,
		ParentStepID: step.ID,
		Data:         data,
	})
}

func (p *Pipeline) runAgentLoop(ctx context.Context, state *State, opts RunOptions) (*State, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	maxIters := opts.LoopMaxIters
	if maxIters <= 0 {
		maxIters = 6
	}
	profile := normalizeLoopProfile(opts)
	ralphEnabled := profile == "ralph"
	loopID := fmt.Sprintf("loop-%d", time.Now().UnixNano())
	sessionID := resolveLoopSessionID(opts, loopID)
	toolNames := opts.ToolNames
	toolHistory := make([]string, 0, maxIters)
	consecutiveFailures := 0
	const maxPromptChars = 14000
	started := time.Now()
	lastIteration := 0
	toolCalls := 0
	toolFailures := 0
	terminationReason := LoopTerminationError
	completionChecks := []CompletionCheckResult{}
	noProgressLimit := opts.NoProgressLimit
	if ralphEnabled && noProgressLimit <= 0 {
		noProgressLimit = 2
	}

	var (
		memoryStore LoopMemoryStore
		memory      = defaultLoopMemorySnapshot(sessionID)
		nonDurable  bool
	)
	if ralphEnabled {
		memoryStore = opts.LoopMemoryStore
		if memoryStore == nil {
			if strings.TrimSpace(opts.LoopMemoryRoot) != "" {
				memoryStore = NewFileLoopMemoryStore(opts.LoopMemoryRoot)
			} else {
				memoryStore = NewInMemoryLoopMemoryStore()
			}
		}
		loaded, _, err := memoryStore.Load(ctx, sessionID)
		if err != nil {
			memoryStore = NewInMemoryLoopMemoryStore()
			loaded, _, err = memoryStore.Load(ctx, sessionID)
			if err != nil {
				terminationReason = LoopTerminationError
				return state, fmt.Errorf("load loop memory fallback: %w", err)
			}
			emitLoopEvent(state, loopEventPayload{
				LoopID:      loopID,
				Iteration:   0,
				AgentStatus: "loop_memory",
				Node:        "loop_memory",
				Status:      "warning",
				Message:     "durable memory backend unavailable, switched to in-memory store",
				DisplayName: "loop memory fallback",
				EventType:   "loop_memory_warning",
			})
		}
		memory = loaded
		memory.SessionID = sessionID
		if len(memory.Checkpoint.ToolHistory) > 0 {
			toolHistory = append(toolHistory, memory.Checkpoint.ToolHistory...)
		}
		if strings.TrimSpace(memory.Checkpoint.LastYAML) != "" && strings.TrimSpace(state.BaseYAML) == "" {
			state.BaseYAML = memory.Checkpoint.LastYAML
		}
		nonDurable = !memoryStore.IsDurable()
		if nonDurable {
			emitLoopEvent(state, loopEventPayload{
				LoopID:      loopID,
				Iteration:   0,
				AgentStatus: "loop_memory",
				Node:        "loop_memory",
				Status:      "warning",
				Message:     "durable memory backend not configured, fallback to in-memory store",
				DisplayName: "loop memory fallback",
				EventType:   "loop_memory_warning",
			})
		}
	}
	saveLoopMemory := func() error {
		if !ralphEnabled || memoryStore == nil {
			return nil
		}
		if err := memoryStore.Save(ctx, sessionID, memory); err != nil {
			if memoryStore.IsDurable() {
				emitLoopEvent(state, loopEventPayload{
					LoopID:      loopID,
					Iteration:   lastIteration,
					AgentStatus: "loop_memory",
					Node:        "loop_memory",
					Status:      "warning",
					Message:     "durable memory save failed, switched to in-memory store",
					DisplayName: "loop memory fallback",
					EventType:   "loop_memory_warning",
				})
				memoryStore = NewInMemoryLoopMemoryStore()
				nonDurable = true
				if fallbackErr := memoryStore.Save(ctx, sessionID, memory); fallbackErr != nil {
					return fmt.Errorf("save loop memory fallback: %w", fallbackErr)
				}
				return nil
			}
			return fmt.Errorf("save loop memory: %w", err)
		}
		return nil
	}
	defer func() {
		if state == nil {
			return
		}
		state.LoopMetrics = &LoopMetrics{
			LoopID:       loopID,
			SessionID:    sessionID,
			ModeProfile:  profile,
			Iterations:   lastIteration,
			ToolCalls:    toolCalls,
			ToolFailures: toolFailures,
			DurationMs:   time.Since(started).Milliseconds(),
			Terminal:     terminationReason,
			Checks:       append([]CompletionCheckResult{}, completionChecks...),
			NonDurable:   nonDurable,
		}
	}()

	for iteration := 1; iteration <= maxIters; iteration++ {
		lastIteration = iteration
		if err := ctx.Err(); err != nil {
			terminationReason = LoopTerminationContextCanceled
			return state, err
		}
		if opts.MaxToolCalls > 0 && toolCalls >= opts.MaxToolCalls {
			terminationReason = LoopTerminationBudgetExceeded
			return state, fmt.Errorf("loop budget exceeded: max tool calls reached")
		}
		if opts.MaxBudgetUnits > 0 && (iteration+toolCalls) > opts.MaxBudgetUnits {
			terminationReason = LoopTerminationBudgetExceeded
			return state, fmt.Errorf("loop budget exceeded: max budget units reached")
		}
		promptText := buildLoopPrompt(
			state.Prompt,
			state.ContextText,
			state.BaseYAML,
			toolNames,
			toolHistory,
			iteration,
			profile,
			opts.CompletionToken,
			opts.CompletionChecks,
			memory.Checkpoint.LastStopReasons,
			memory.Progress,
		)
		if len(promptText) > maxPromptChars {
			terminationReason = LoopTerminationError
			return state, fmt.Errorf("loop prompt too long")
		}
		messages := []ai.Message{
			{Role: "system", Content: state.SystemPrompt},
			{Role: "user", Content: promptText},
		}
		iterCtx := ctx
		cancelIter := func() {}
		if opts.PerIterTimeoutMs > 0 {
			iterCtx, cancelIter = context.WithTimeout(ctx, time.Duration(opts.PerIterTimeoutMs)*time.Millisecond)
		}
		reply, thought, err := p.chatWithThought(ai.WithModelRole(iterCtx, ai.RolePlanner), messages, state.StreamSink)
		cancelIter()
		if err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				terminationReason = LoopTerminationContextCanceled
				return state, ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				terminationReason = LoopTerminationBudgetExceeded
				return state, fmt.Errorf("loop iteration timeout")
			}
			consecutiveFailures++
			if consecutiveFailures >= 2 {
				terminationReason = LoopTerminationError
				return state, err
			}
			continue
		}
		state.Thought = strings.TrimSpace(thought)
		action, err := parseLoopAction(reply)
		if err != nil {
			consecutiveFailures++
			if consecutiveFailures >= 2 {
				terminationReason = LoopTerminationError
				return state, err
			}
			continue
		}

		switch normalizeLoopAction(action.Action) {
		case "final":
			candidateYAML := ""
			candidateQuestions := append([]string{}, action.Questions...)
			if yamlText := strings.TrimSpace(action.YAML); yamlText != "" {
				candidateYAML = normalizeStepsOnlyYAML(yamlText)
				if strings.TrimSpace(candidateYAML) == "" {
					terminationReason = LoopTerminationError
					return state, fmt.Errorf("loop final output missing steps")
				}
			}
			if candidateYAML == "" && action.Result != "" {
				yamlText, questions, err := extractWorkflowYAML(action.Result)
				if err == nil {
					candidateYAML = yamlText
					candidateQuestions = mergeQuestions(candidateQuestions, questions)
				}
			}
			if candidateYAML == "" {
				yamlText, questions, err := extractWorkflowYAML(reply)
				if err == nil {
					candidateYAML = yamlText
					candidateQuestions = mergeQuestions(candidateQuestions, questions)
				}
			}
			if strings.TrimSpace(candidateYAML) == "" {
				terminationReason = LoopTerminationError
				return state, fmt.Errorf("loop final output missing yaml")
			}
			state.YAML = candidateYAML
			state.Questions = mergeQuestions(state.Questions, candidateQuestions)

			if !ralphEnabled {
				terminationReason = LoopTerminationCompleted
				return state, nil
			}

			evaluation := evaluateLoopCompletion(state, action, reply, opts, memory)
			completionChecks = append([]CompletionCheckResult{}, evaluation.CheckResults...)
			if evaluation.Passed {
				terminationReason = LoopTerminationCompleted
				memory.Checkpoint.Iteration = iteration
				memory.Checkpoint.ToolCalls = toolCalls
				memory.Checkpoint.ToolFailures = toolFailures
				memory.Checkpoint.LastYAML = state.YAML
				memory.Checkpoint.ToolHistory = append([]string{}, tailStrings(toolHistory, 32)...)
				appendLoopProgress(&memory, fmt.Sprintf("[%s] iteration=%d final accepted", time.Now().Format(time.RFC3339), iteration))
				if err := saveLoopMemory(); err != nil {
					terminationReason = LoopTerminationError
					return state, err
				}
				return state, nil
			}

			memory.Checkpoint.LastStopReasons = append([]string{}, evaluation.Failed...)
			appendLoopProgress(&memory, fmt.Sprintf("[%s] iteration=%d final blocked: %s", time.Now().Format(time.RFC3339), iteration, strings.Join(evaluation.Failed, "; ")))
			emitLoopEvent(state, loopEventPayload{
				LoopID:      loopID,
				Iteration:   iteration,
				AgentStatus: "completion_blocked",
				Node:        "stop_hook",
				Status:      "blocked",
				Message:     strings.Join(evaluation.Failed, "; "),
				DisplayName: "stop hook blocked completion",
				EventType:   "completion_blocked",
				Data: map[string]any{
					"failed_checks": evaluation.Failed,
				},
			})

			fingerprint := computeLoopFingerprint(action, state.YAML, tailStrings(toolHistory, 4), completionChecks)
			if updateNoProgress(&memory.Checkpoint, fingerprint, noProgressLimit) {
				terminationReason = LoopTerminationNoProgress
				if err := saveLoopMemory(); err != nil {
					return state, err
				}
				return state, fmt.Errorf("loop no progress detected")
			}
			memory.Checkpoint.Iteration = iteration
			memory.Checkpoint.ToolCalls = toolCalls
			memory.Checkpoint.ToolFailures = toolFailures
			memory.Checkpoint.LastYAML = state.YAML
			memory.Checkpoint.ToolHistory = append([]string{}, tailStrings(toolHistory, 32)...)
			if err := saveLoopMemory(); err != nil {
				terminationReason = LoopTerminationError
				return state, err
			}
			consecutiveFailures = 0
			toolHistory = append(toolHistory, fmt.Sprintf("stop_hook_blocked=%s", strings.Join(evaluation.Failed, "; ")))
			continue
		case "need_more_info":
			if len(action.Questions) > 0 {
				state.Questions = mergeQuestions(state.Questions, action.Questions)
			}
			terminationReason = LoopTerminationCompleted
			return state, nil
		case "tool_call":
			toolName := strings.TrimSpace(action.Tool)
			if toolName == "" {
				terminationReason = LoopTerminationError
				return state, fmt.Errorf("tool_call missing tool name")
			}
			if opts.ToolExecutor == nil {
				terminationReason = LoopTerminationError
				return state, fmt.Errorf("tool executor is not configured")
			}
			if opts.MaxToolCalls > 0 && toolCalls >= opts.MaxToolCalls {
				terminationReason = LoopTerminationBudgetExceeded
				return state, fmt.Errorf("loop budget exceeded: max tool calls reached")
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
				EventType:   "tool_call",
			})
			toolCtx := ctx
			cancelTool := func() {}
			if opts.PerIterTimeoutMs > 0 {
				toolCtx, cancelTool = context.WithTimeout(ctx, time.Duration(opts.PerIterTimeoutMs)*time.Millisecond)
			}
			output, err := opts.ToolExecutor(toolCtx, toolName, action.Args)
			cancelTool()
			if err != nil {
				if errors.Is(err, context.Canceled) && ctx.Err() != nil {
					terminationReason = LoopTerminationContextCanceled
					return state, ctx.Err()
				}
				if errors.Is(err, context.DeadlineExceeded) {
					terminationReason = LoopTerminationBudgetExceeded
					return state, fmt.Errorf("tool call timeout")
				}
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
					EventType: "tool_result",
				})
				consecutiveFailures++
				if consecutiveFailures >= 2 {
					terminationReason = LoopTerminationError
					return state, err
				}
				toolHistory = append(toolHistory, fmt.Sprintf("tool=%s error=%s", toolName, err.Error()))
				if ralphEnabled {
					appendLoopProgress(&memory, fmt.Sprintf("[%s] iteration=%d tool=%s error=%s", time.Now().Format(time.RFC3339), iteration, toolName, err.Error()))
					memory.Checkpoint.Iteration = iteration
					memory.Checkpoint.ToolCalls = toolCalls
					memory.Checkpoint.ToolFailures = toolFailures
					memory.Checkpoint.ToolHistory = append([]string{}, tailStrings(toolHistory, 32)...)
					if err := saveLoopMemory(); err != nil {
						terminationReason = LoopTerminationError
						return state, err
					}
				}
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
				EventType: "tool_result",
			})
			summary := ai.SummarizeToolOutput(output, 400)
			toolHistory = append(toolHistory, fmt.Sprintf("tool=%s output=%s", toolName, summary))
			if ralphEnabled {
				appendLoopProgress(&memory, fmt.Sprintf("[%s] iteration=%d tool=%s output=%s", time.Now().Format(time.RFC3339), iteration, toolName, summary))
				fingerprint := computeLoopFingerprint(action, state.YAML, tailStrings(toolHistory, 4), completionChecks)
				if updateNoProgress(&memory.Checkpoint, fingerprint, noProgressLimit) {
					terminationReason = LoopTerminationNoProgress
					if err := saveLoopMemory(); err != nil {
						return state, err
					}
					return state, fmt.Errorf("loop no progress detected")
				}
				memory.Checkpoint.Iteration = iteration
				memory.Checkpoint.ToolCalls = toolCalls
				memory.Checkpoint.ToolFailures = toolFailures
				memory.Checkpoint.ToolHistory = append([]string{}, tailStrings(toolHistory, 32)...)
				if err := saveLoopMemory(); err != nil {
					terminationReason = LoopTerminationError
					return state, err
				}
			}
			continue
		default:
			terminationReason = LoopTerminationError
			return state, fmt.Errorf("unknown loop action")
		}
	}
	terminationReason = LoopTerminationMaxIters
	return state, fmt.Errorf("loop max iterations reached")
}

func normalizeLoopProfile(opts RunOptions) string {
	if opts.RalphMode || strings.EqualFold(strings.TrimSpace(opts.LoopProfile), "ralph") {
		return "ralph"
	}
	return "default"
}

func resolveLoopSessionID(opts RunOptions, fallback string) string {
	if id := strings.TrimSpace(opts.ResumeCheckpointID); id != "" {
		return normalizeLoopSessionID(id)
	}
	if id := strings.TrimSpace(opts.SessionKey); id != "" {
		return normalizeLoopSessionID(id)
	}
	if id := strings.TrimSpace(opts.DraftID); id != "" {
		return normalizeLoopSessionID(id)
	}
	return normalizeLoopSessionID(fallback)
}

func appendLoopProgress(snapshot *LoopMemorySnapshot, line string) {
	if snapshot == nil || strings.TrimSpace(line) == "" {
		return
	}
	if snapshot.Progress != "" && !strings.HasSuffix(snapshot.Progress, "\n") {
		snapshot.Progress += "\n"
	}
	snapshot.Progress += strings.TrimSpace(line) + "\n"
}

func computeLoopFingerprint(action loopAction, yamlText string, history []string, checks []CompletionCheckResult) string {
	lastHistory := ""
	if len(history) > 0 {
		lastHistory = strings.TrimSpace(history[len(history)-1])
	}
	payload := map[string]any{
		"action":       strings.ToLower(strings.TrimSpace(action.Action)),
		"tool":         strings.ToLower(strings.TrimSpace(action.Tool)),
		"yaml":         strings.TrimSpace(stepsOnlyYAML(yamlText)),
		"last_history": lastHistory,
		"checks":       checks,
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func updateNoProgress(checkpoint *LoopCheckpoint, fingerprint string, limit int) bool {
	if checkpoint == nil || limit <= 0 || strings.TrimSpace(fingerprint) == "" {
		return false
	}
	if checkpoint.LastFingerprint == fingerprint {
		checkpoint.StableIterations++
	} else {
		checkpoint.StableIterations = 0
	}
	checkpoint.LastFingerprint = fingerprint
	return checkpoint.StableIterations >= limit
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
	EventType   string
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
		EventType:   payload.EventType,
		LoopID:      payload.LoopID,
		Iteration:   payload.Iteration,
		AgentStatus: payload.AgentStatus,
		Data:        payload.Data,
	})
}
