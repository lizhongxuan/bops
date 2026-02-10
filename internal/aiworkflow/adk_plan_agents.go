package aiworkflow

import (
	"context"
	"encoding/json"
	"strings"

	"bops/runner/logging"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type bopsPlan struct {
	Steps []PlanStep `json:"steps"`
}

func init() {
	schema.RegisterName[*bopsPlan]("bops_plan")
}

func (p *bopsPlan) FirstStep() string {
	if p == nil || len(p.Steps) == 0 {
		return ""
	}
	return p.Steps[0].StepName
}

func (p *bopsPlan) MarshalJSON() ([]byte, error) {
	type alias bopsPlan
	return json.Marshal((*alias)(p))
}

func (p *bopsPlan) UnmarshalJSON(data []byte) error {
	var steps []PlanStep
	if err := json.Unmarshal(data, &steps); err == nil {
		p.Steps = normalizePlanSteps(steps)
		return nil
	}
	var payload planPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	p.Steps = normalizePlanSteps(payload.Steps)
	return nil
}

type bopsPlannerAgent struct {
	pipeline     *Pipeline
	state        *State
	systemPrompt string
	store        *DraftStore
	draftID      string
}

func (a *bopsPlannerAgent) Name(_ context.Context) string {
	return "planner"
}

func (a *bopsPlannerAgent) Description(_ context.Context) string {
	return "bops planner agent"
}

func (a *bopsPlannerAgent) Run(ctx context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer generator.Close()
		if a == nil || a.pipeline == nil || a.state == nil {
			generator.Send(&adk.AgentEvent{Err: context.Canceled})
			return
		}
		prompt := extractLastUserPrompt(input.Messages)
		if prompt != "" {
			a.state.Prompt = prompt
		}

		plan, missing, err := a.pipeline.runCoordinator(ctx, a.state, a.systemPrompt)
		if err != nil {
			generator.Send(&adk.AgentEvent{Err: err})
			return
		}

		askQuestions := len(missing) > 0 && shouldAskQuestions(a.state.Prompt)
		if askQuestions {
			if len(plan) == 0 {
				plan = buildFallbackPlan(a.state.Prompt)
			}
			a.state.Plan = plan
			if a.store != nil {
				a.store.UpdatePlan(a.draftID, plan)
			}
			emitCustomEvent(a.state, "plan_ready", "done", "plan ready", map[string]any{
				"plan": plan,
			})
			emitPlanSteps(a.state)
			a.state.Questions = buildQuestionsFromMissing(missing)
			emitEvent(a.state, "question_gate", "done", "awaiting missing inputs")
			generator.Send(adk.StatefulInterrupt(ctx, "missing_info", &missingInfo{Missing: missing}))
			return
		}

		if len(plan) == 0 {
			plan = buildFallbackPlan(a.state.Prompt)
		}
		a.state.Plan = plan
		if a.store != nil {
			a.store.UpdatePlan(a.draftID, plan)
		}
		emitCustomEvent(a.state, "plan_ready", "done", "plan ready", map[string]any{
			"plan": plan,
		})

		adk.AddSessionValue(ctx, planexecute.PlanSessionKey, &bopsPlan{Steps: plan})
		adk.AddSessionValue(ctx, planexecute.UserInputSessionKey, input.Messages)
		if _, ok := adk.GetSessionValue(ctx, planexecute.ExecutedStepsSessionKey); !ok {
			adk.AddSessionValue(ctx, planexecute.ExecutedStepsSessionKey, []planexecute.ExecutedStep{})
		}

		payload, _ := json.Marshal(&bopsPlan{Steps: plan})
		msg := schema.AssistantMessage(string(payload), nil)
		generator.Send(adk.EventFromMessage(msg, nil, schema.Assistant, ""))
	}()
	return iterator
}

type bopsReplannerAgent struct {
	state   *State
	store   *DraftStore
	draftID string
}

func (a *bopsReplannerAgent) Name(_ context.Context) string {
	return "replanner"
}

func (a *bopsReplannerAgent) Description(_ context.Context) string {
	return "bops replanner"
}

func (a *bopsReplannerAgent) Run(ctx context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer generator.Close()
		if a == nil {
			return
		}
		planValue, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey)
		if !ok {
			return
		}
		plan, ok := planValue.(*bopsPlan)
		if !ok || plan == nil {
			return
		}
		executed := []planexecute.ExecutedStep{}
		if value, ok := adk.GetSessionValue(ctx, planexecute.ExecutedStepsSessionKey); ok {
			if casted, ok := value.([]planexecute.ExecutedStep); ok {
				executed = casted
			}
		}
		if len(executed) < len(plan.Steps) {
			return
		}

		if a.store != nil && a.state != nil {
			if snapshot := a.store.Snapshot(a.draftID); snapshot.DraftID != "" {
				if yamlText, err := buildFinalYAML(snapshot); err == nil {
					a.state.YAML = yamlText
				}
			}
			emitCustomEvent(a.state, "finalize_success", "done", "workflow created", nil)
		}
		generator.Send(adk.EventFromMessage(schema.AssistantMessage("工作流生成完成。", nil), nil, schema.Assistant, ""))
		generator.Send(&adk.AgentEvent{Action: adk.NewBreakLoopAction(a.Name(ctx))})
	}()
	return iterator
}

func extractLastUserPrompt(messages []adk.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg == nil {
			continue
		}
		if msg.Role == schema.User {
			return strings.TrimSpace(msg.Content)
		}
	}
	return ""
}

func buildExecutorInputFn(state *State, systemPrompt string, contextText string) planexecute.GenModelInputFn {
	return func(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
		planValue, ok := in.Plan.(*bopsPlan)
		if !ok || planValue == nil || len(planValue.Steps) == 0 {
			return []adk.Message{schema.SystemMessage(systemPrompt), schema.UserMessage("No steps available.")}, nil
		}
		stepIndex := len(in.ExecutedSteps)
		if stepIndex >= len(planValue.Steps) {
			return []adk.Message{schema.SystemMessage(systemPrompt), schema.UserMessage("All steps are completed.")}, nil
		}
		step := planValue.Steps[stepIndex]
		emitPlanStepStartIfNeeded(state, step)
		prompt := buildCoderToolPrompt(step, contextText)
		return []adk.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(prompt),
		}, nil
	}
}

func emitPlanStepStartIfNeeded(state *State, step PlanStep) {
	if state == nil {
		return
	}
	if step.ID == "" {
		step.ID = normalizePlanID(step.StepName, 0)
	}
	if state.StepStatuses != nil {
		if status, ok := state.StepStatuses[step.ID]; ok && status == StepStatusInProgress {
			return
		}
	}
	state.AgentName = "Coder"
	state.AgentRole = "coder"
	setStepStatus(state, step.ID, StepStatusInProgress)
	emitStepEvent(state, "plan_step_start", step, "start", "plan step", nil)
}

func buildCoderToolPrompt(step PlanStep, contextText string) string {
	builder := strings.Builder{}
	builder.WriteString("You are a workflow coder. Use the tool step_patch to submit a single step.\n")
	builder.WriteString("Respond with JSON only in this envelope:\n")
	builder.WriteString("{\"tool\":\"step_patch\",\"args\":{\"step_id\":\"...\",\"step_name\":\"...\",\"action\":\"...\",\"targets\":[],\"args\":{},\"summary\":\"...\"}}\n")
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
	builder.WriteString("\n\nReturn JSON only. Do not include markdown or explanations.")
	return builder.String()
}

func logPlanSteps(plan []PlanStep) {
	if len(plan) == 0 {
		return
	}
	for _, step := range plan {
		logging.L().Debug("plan step",
			zap.String("step_id", step.ID),
			zap.String("step_name", step.StepName),
			zap.String("status", string(step.Status)),
		)
	}
}
