package aiworkflow

import (
	"context"
	"encoding/json"
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

type coordinatorPayload struct {
	Plan    []PlanStep `json:"plan"`
	Missing []string   `json:"missing,omitempty"`
}

func (p *Pipeline) runMultiCreateLegacy(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	if p == nil {
		return nil, errors.New("pipeline is nil")
	}
	systemPrompt := pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt)
	state := &State{
		Mode:          ModeGenerate,
		Prompt:        prompt,
		Context:       context,
		ContextText:   opts.ContextText,
		SystemPrompt:  systemPrompt,
		BaseYAML:      opts.BaseYAML,
		MaxRetries:    pickMaxRetries(opts.MaxRetries, p.cfg.MaxRetries),
		ValidationEnv: opts.ValidationEnv,
		SkipExecute:   opts.SkipExecute,
		EventSink:     opts.EventSink,
		StreamSink:    opts.StreamSink,
	}
	draftID := strings.TrimSpace(opts.DraftID)
	if draftID == "" {
		draftID = strings.TrimSpace(opts.SessionKey)
	}
	if draftID == "" {
		draftID = fmt.Sprintf("draft-%d", time.Now().UnixNano())
	}
	draftStore := p.draftStore
	draftStore.GetOrCreate(draftID, opts.BaseYAML)

	logging.L().Info("multi-create start",
		zap.String("draft_id", draftID),
		zap.Int("prompt_len", len(prompt)),
	)

	plan, missing, err := p.runCoordinator(ctx, state, systemPrompt)
	if err != nil {
		emitCustomEvent(state, "coordinator_error", "error", err.Error(), nil)
		return state, err
	}
	askQuestions := len(missing) > 0 && shouldAskQuestions(state.Prompt)
	if askQuestions {
		if len(plan) == 0 {
			plan = buildFallbackPlan(state.Prompt)
		}
		state.Plan = plan
		draftStore.UpdatePlan(draftID, plan)
		emitCustomEvent(state, "plan_ready", "done", "plan ready", map[string]any{
			"plan": plan,
		})
		emitPlanSteps(state)
		state.Questions = buildQuestionsFromMissing(missing)
		emitEvent(state, "question_gate", "done", "awaiting missing inputs")
		return state, nil
	}
	if len(missing) > 0 {
		logging.L().Info("coordinator missing ignored",
			zap.Int("missing", len(missing)),
		)
	}
	if len(plan) == 0 {
		plan = buildFallbackPlan(state.Prompt)
	}
	state.Plan = plan
	draftStore.UpdatePlan(draftID, plan)
	emitCustomEvent(state, "plan_ready", "done", "plan ready", map[string]any{
		"plan": plan,
	})

	for i := range state.Plan {
		step := state.Plan[i]
		if strings.TrimSpace(step.ID) == "" {
			step.ID = normalizePlanID(step.StepName, i)
			state.Plan[i].ID = step.ID
		}
		state.Plan[i].Status = StepStatusInProgress
		setStepStatus(state, step.ID, StepStatusInProgress)
		state.AgentName = "Coordinator"
		state.AgentRole = "coordinator"
		emitStepEvent(state, "plan_step_start", step, "start", "plan step", nil)

		state.AgentName = "Coder"
		state.AgentRole = "coder"
		patch, err := p.runCoderStep(ctx, step, state, systemPrompt)
		if err != nil {
			state.Plan[i].Status = StepStatusFailed
			setStepStatus(state, step.ID, StepStatusFailed)
			emitStepEvent(state, "step_patch_created", step, "error", err.Error(), nil)
			return state, err
		}
		if patch.StepID == "" {
			patch.StepID = step.ID
		}
		if patch.StepName == "" {
			patch.StepName = step.StepName
		}
		patch = alignStepPatchWithPlan(draftStore, draftID, patch)
		patch.Source = "coder"
		draftStore.UpdateStep(draftID, patch)
		var yamlText string
		if snapshot := draftStore.Snapshot(draftID); snapshot.DraftID != "" {
			if next, err := buildFinalYAML(snapshot); err == nil {
				yamlText = next
			}
		}
		emitStepEvent(state, "step_patch_created", step, "done", patch.Summary, map[string]any{
			"step_patch": patch,
			"yaml":       yamlText,
		})

		reviewResult := p.reviewStep(ctx, state, draftStore, draftID, ReviewTask{
			StepID: patch.StepID,
			Patch:  patch,
			Status: "pending",
		}, opts)
		if reviewResult.Status == StepStatusFailed {
			state.Plan[i].Status = StepStatusFailed
			setStepStatus(state, step.ID, StepStatusFailed)
			emitStepEvent(state, "plan_step_done", step, "error", reviewResult.Summary, nil)
			continue
		}
		state.Plan[i].Status = StepStatusDone
		setStepStatus(state, step.ID, StepStatusDone)
		emitStepEvent(state, "plan_step_done", step, "done", reviewResult.Summary, nil)
	}
	emitCustomEvent(state, "coder_done", "done", "coder completed", nil)

	finalYAML, err := buildFinalYAML(draftStore.Snapshot(draftID))
	if err != nil {
		emitCustomEvent(state, "finalize_failed", "error", err.Error(), nil)
		return state, err
	}
	state.YAML = finalYAML

	emitCustomEvent(state, "final_validation", "start", "final validation", nil)
	validatorState := *state
	if _, err := p.validate(ctx, &validatorState); err != nil {
		state.Issues = []string{err.Error()}
	}
	state.Issues = validatorState.Issues

	if len(state.Issues) > 0 {
		fixOpts := opts
		fixOpts.MaxRetries = 1
		fixOpts.SkipExecute = true
		fixState, fixErr := p.RunFix(ctx, state.YAML, state.Issues, fixOpts)
		if fixErr == nil && fixState != nil && strings.TrimSpace(fixState.YAML) != "" {
			state.YAML = fixState.YAML
		}
		validatorState = *state
		_, _ = p.validate(ctx, &validatorState)
		state.Issues = validatorState.Issues
		if len(state.Issues) > 0 {
			emitCustomEvent(state, "finalize_failed", "error", "final validation failed", map[string]any{"issues": state.Issues})
			return state, nil
		}
	}
	emitCustomEvent(state, "finalize_success", "done", "workflow created", nil)
	return state, nil
}

func emitPlanSteps(state *State) {
	if state == nil {
		return
	}
	for i := range state.Plan {
		step := &state.Plan[i]
		if strings.TrimSpace(step.ID) == "" {
			step.ID = normalizePlanID(step.StepName, i)
		}
		if step.Status == "" {
			step.Status = StepStatusPending
		}
		state.AgentName = "Coordinator"
		state.AgentRole = "coordinator"
		emitStepEvent(state, "plan_step_start", *step, "start", "plan step", nil)
		emitStepEvent(state, "plan_step_done", *step, "done", "plan step", nil)
	}
}

func buildFallbackPlan(prompt string) []PlanStep {
	desc := strings.TrimSpace(prompt)
	if desc == "" {
		desc = "生成可执行的工作流步骤"
	}
	steps := []PlanStep{
		{
			StepName:    "生成工作流步骤",
			Description: desc,
		},
	}
	return normalizePlanSteps(steps)
}

func shouldAskQuestions(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return false
	}
	normalized := strings.ToLower(trimmed)
	triggers := []string{
		"需要补充",
		"需要什么信息",
		"需要哪些信息",
		"你需要什么",
		"你需要哪些",
		"缺什么信息",
		"还缺什么",
		"还需要什么",
		"请问我需要",
		"请问需要",
		"你还要",
		"请问还要",
		"不清楚",
		"不确定",
		"不知道",
		"你问我",
		"问我",
		"请提问",
		"请先确认",
		"帮我确认",
		"请确认",
		"需要确认",
		"确认一下",
		"需要哪些参数",
		"要哪些参数",
		"ask me",
		"what info",
		"what information",
		"need more info",
		"missing info",
	}
	for _, trigger := range triggers {
		if strings.Contains(normalized, trigger) {
			return true
		}
	}
	return false
}

func (p *Pipeline) runCoordinator(ctx context.Context, state *State, systemPrompt string) ([]PlanStep, []string, error) {
	if p.cfg.Client == nil {
		return nil, nil, errors.New("ai client is not configured")
	}
	state.AgentName = "Coordinator"
	state.AgentRole = "coordinator"
	prompt := buildCoordinatorPrompt(state.Prompt, state.ContextText)
	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}
	reply, _, err := p.chatWithThought(ctx, messages, state.StreamSink)
	if err != nil {
		return nil, nil, err
	}
	payload, err := parseCoordinatorPayload(reply)
	if err != nil {
		return nil, nil, err
	}
	if len(payload.Missing) > 0 {
		return nil, normalizeMissing(payload.Missing), nil
	}
	return normalizePlanSteps(payload.Plan), nil, nil
}

func buildCoordinatorPrompt(prompt, contextText string) string {
	builder := strings.Builder{}
	builder.WriteString("You are a workflow coordinator. Return JSON only.\n")
	builder.WriteString("Output format: {\"plan\":[{\"step_name\":\"...\",\"description\":\"...\",\"dependencies\":[]}],\"missing\":[]}\n")
	builder.WriteString("Do not ask for missing information unless the user explicitly requests questions; assume reasonable defaults and keep missing empty.\n")
	if contextText != "" {
		builder.WriteString("Context:\n")
		builder.WriteString(contextText)
		builder.WriteString("\n\n")
	}
	builder.WriteString("User request:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\n")
	builder.WriteString("If information is missing, fill missing[] and keep plan minimal.\n")
	builder.WriteString("Do not include markdown or explanations.")
	return builder.String()
}

func parseCoordinatorPayload(reply string) (coordinatorPayload, error) {
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		return coordinatorPayload{}, errors.New("coordinator response is not json")
	}
	var payload coordinatorPayload
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return coordinatorPayload{}, err
	}
	return payload, nil
}

func (p *Pipeline) runCoderStep(ctx context.Context, step PlanStep, state *State, systemPrompt string) (StepPatch, error) {
	if p.cfg.Client == nil {
		return StepPatch{}, errors.New("ai client is not configured")
	}
	state.AgentName = "Coder"
	state.AgentRole = "coder"
	prompt := buildCoderPrompt(step, state.ContextText)
	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}
	reply, _, err := p.chatWithThought(ctx, messages, state.StreamSink)
	if err != nil {
		return StepPatch{}, err
	}
	patch, err := parseStepPatchJSON(reply)
	if err != nil {
		return StepPatch{}, err
	}
	if patch.StepID == "" {
		patch.StepID = step.ID
	}
	if patch.StepName == "" {
		patch.StepName = step.StepName
	}
	return patch, nil
}

func buildCoderPrompt(step PlanStep, contextText string) string {
	builder := strings.Builder{}
	builder.WriteString("You are a workflow coder. Return JSON only.\n")
	builder.WriteString("Output format: {\"step_id\":\"...\",\"step_name\":\"...\",\"action\":\"...\",\"targets\":[],\"with\":{},\"summary\":\"...\"}\n")
	builder.WriteString("Allowed actions: ")
	builder.WriteString(allowedActionText())
	builder.WriteString(".\n")
	if contextText != "" {
		builder.WriteString("Context:\n")
		builder.WriteString(contextText)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Plan step:\n")
	builder.WriteString("- id: ")
	builder.WriteString(strings.TrimSpace(step.ID))
	builder.WriteString("\n- name: ")
	builder.WriteString(strings.TrimSpace(step.StepName))
	if strings.TrimSpace(step.Description) != "" {
		builder.WriteString("\n- description: ")
		builder.WriteString(strings.TrimSpace(step.Description))
	}
	builder.WriteString("\n\nReturn JSON only. Do not include markdown.")
	return builder.String()
}

func (p *Pipeline) reviewWorker(ctx context.Context, state *State, store *DraftStore, draftID string, tasks <-chan ReviewTask, opts RunOptions) {
	for task := range tasks {
		p.reviewStep(ctx, state, store, draftID, task, opts)
	}
}

func (p *Pipeline) reviewStep(ctx context.Context, state *State, store *DraftStore, draftID string, task ReviewTask, opts RunOptions) ReviewResult {
	reviewStart := time.Now()
	stepID := task.StepID
	state.AgentName = "Reviewer"
	state.AgentRole = "reviewer"
	emitCustomEvent(state, "review_start", "start", "review start", map[string]any{
		"step_id":   stepID,
		"step_name": task.Patch.StepName,
	})
	patch := task.Patch
	issues := validateStepPatch(patch)
	if len(issues) > 0 {
		fixed, err := p.runReviewerFix(ctx, patch, issues, "", opts)
		if err == nil {
			patch = fixed
			issues = validateStepPatch(patch)
			emitCustomEvent(state, "review_update", "done", patch.Summary, map[string]any{
				"step_id":   stepID,
				"step_name": patch.StepName,
			})
		}
	}
	execIssues := []string{}
	if opts.ValidationEnv != nil && !opts.SkipExecute {
		emitCustomEvent(state, "validation_start", "start", "validation start", map[string]any{
			"step_id":   stepID,
			"step_name": patch.StepName,
		})
		validationStart := time.Now()
		execIssues = p.runStepValidation(ctx, patch, opts)
		store.AddMetric(draftID, "validation_duration_ms", int(time.Since(validationStart).Milliseconds()))
		emitCustomEvent(state, "validation_done", "done", "validation done", map[string]any{
			"step_id":   stepID,
			"step_name": patch.StepName,
			"issues":    execIssues,
		})
		if len(execIssues) > 0 {
			for attempt := 0; attempt < 3 && len(execIssues) > 0; attempt++ {
				store.AddMetric(draftID, "validation_retries", 1)
				fixed, err := p.runReviewerFix(ctx, patch, execIssues, strings.Join(execIssues, "; "), opts)
				if err != nil {
					break
				}
				patch = fixed
				emitCustomEvent(state, "review_update", "done", patch.Summary, map[string]any{
					"step_id":   stepID,
					"step_name": patch.StepName,
				})
				validationStart := time.Now()
				execIssues = p.runStepValidation(ctx, patch, opts)
				store.AddMetric(draftID, "validation_duration_ms", int(time.Since(validationStart).Milliseconds()))
			}
		}
	}
	patch = alignStepPatchWithPlan(store, draftID, patch)
	store.UpdateStep(draftID, patch)
	result := ReviewResult{
		StepID:   stepID,
		Status:   StepStatusDone,
		Summary:  patch.Summary,
		Issues:   append(issues, execIssues...),
		Attempts: task.Attempt + 1,
	}
	if len(result.Issues) > 0 {
		result.Status = StepStatusFailed
	}
	store.UpdateReview(draftID, result)
	store.AddMetric(draftID, "review_duration_ms", int(time.Since(reviewStart).Milliseconds()))
	if task.Attempt > 0 {
		store.AddMetric(draftID, "review_retries", task.Attempt)
	}
	var yamlText string
	if snapshot := store.Snapshot(draftID); snapshot.DraftID != "" {
		if next, err := buildFinalYAML(snapshot); err == nil {
			yamlText = next
		}
	}
	logging.L().Info("review done",
		zap.String("draft_id", draftID),
		zap.String("step_id", stepID),
		zap.String("status", string(result.Status)),
		zap.Int("issues", len(result.Issues)),
		zap.Duration("duration", time.Since(reviewStart)),
	)
	status := "done"
	if result.Status == StepStatusFailed {
		status = "error"
	}
	emitCustomEvent(state, "review_done", status, patch.Summary, map[string]any{
		"step_id":    stepID,
		"step_name":  patch.StepName,
		"issues":     result.Issues,
		"step_patch": patch,
		"yaml":       yamlText,
	})
	return result
}

func (p *Pipeline) runReviewerFix(ctx context.Context, patch StepPatch, issues []string, execError string, opts RunOptions) (StepPatch, error) {
	if p.cfg.Client == nil {
		return StepPatch{}, errors.New("ai client is not configured")
	}
	prompt := buildReviewerPrompt(patch, issues, execError)
	messages := []ai.Message{
		{Role: "system", Content: pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt)},
		{Role: "user", Content: prompt},
	}
	reply, _, err := p.chatWithThought(ctx, messages, nil)
	if err != nil {
		return StepPatch{}, err
	}
	fixed, err := parseStepPatchJSON(reply)
	if err != nil {
		return StepPatch{}, err
	}
	if fixed.StepID == "" {
		fixed.StepID = patch.StepID
	}
	if fixed.StepName == "" {
		fixed.StepName = patch.StepName
	}
	fixed.Source = "reviewer"
	return fixed, nil
}

func buildReviewerPrompt(patch StepPatch, issues []string, execError string) string {
	builder := strings.Builder{}
	builder.WriteString("You are a workflow reviewer. Fix the step JSON and return JSON only.\n")
	builder.WriteString("Output format: {\"step_id\":\"...\",\"step_name\":\"...\",\"action\":\"...\",\"targets\":[],\"with\":{},\"summary\":\"...\"}\n")
	builder.WriteString("Allowed actions: ")
	builder.WriteString(allowedActionText())
	builder.WriteString(".\n\n")
	if len(issues) > 0 {
		builder.WriteString("Issues:\n")
		for _, issue := range issues {
			builder.WriteString("- ")
			builder.WriteString(issue)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	if strings.TrimSpace(execError) != "" {
		builder.WriteString("Execution error:\n")
		builder.WriteString(execError)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Current step:\n")
	raw, _ := json.Marshal(patch)
	builder.WriteString(string(raw))
	builder.WriteString("\n\nReturn JSON only.")
	return builder.String()
}

func (p *Pipeline) runStepValidation(ctx context.Context, patch StepPatch, opts RunOptions) []string {
	step := workflow.Step{
		Name:    patch.StepName,
		Targets: patch.Targets,
		Action:  patch.Action,
		With:    patch.With,
	}
	wf := normalizeWorkflow(workflow.Workflow{Steps: []workflow.Step{step}})
	yamlText, err := marshalWorkflowYAML(wf)
	if err != nil {
		return []string{err.Error()}
	}
	if opts.ValidationEnv == nil {
		return nil
	}
	result, err := validationrun.Runner(ctx, *opts.ValidationEnv, yamlText)
	if err != nil {
		return []string{err.Error()}
	}
	if result.Status != "success" {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" {
			msg = "validation failed"
		}
		return []string{msg}
	}
	return nil
}

func buildFinalYAML(snapshot DraftState) (string, error) {
	steps := make([]workflow.Step, 0, len(snapshot.Plan))
	for _, step := range snapshot.Plan {
		patch, ok := snapshot.Steps[step.ID]
		if !ok {
			continue
		}
		steps = append(steps, workflow.Step{
			Name:    patch.StepName,
			Targets: patch.Targets,
			Action:  patch.Action,
			With:    patch.With,
		})
	}
	wf := normalizeWorkflow(workflow.Workflow{Steps: steps})
	finalYAML, err := marshalWorkflowYAML(wf)
	if err != nil {
		return "", err
	}
	if snapshot.BaseYAML != "" {
		stepsOnly := stepsOnlyYAML(finalYAML)
		if stepsOnly != "" {
			return mergeStepsIntoBase(snapshot.BaseYAML, stepsOnly), nil
		}
	}
	return finalYAML, nil
}

func emitCustomEvent(state *State, eventType, status, message string, data map[string]any) {
	if state == nil || state.EventSink == nil {
		return
	}
	if data == nil {
		data = map[string]any{}
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
		ParentStepID: "",
		Data:         data,
	})
}
