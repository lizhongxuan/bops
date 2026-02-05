package aiworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bops/internal/logging"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type stepPatchTool struct {
	pipeline *Pipeline
	state    *State
	store    *DraftStore
	draftID  string
	opts     RunOptions
}

func (t *stepPatchTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "step_patch",
		Desc: "Create or update a workflow step. Use this tool to submit a single step patch.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"step_id": {
				Type: schema.String,
				Desc: "Unique step id",
			},
			"step_name": {
				Type:     schema.String,
				Desc:     "Step name",
				Required: true,
			},
			"action": {
				Type:     schema.String,
				Desc:     "Workflow action",
				Required: true,
			},
			"targets": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "Target hosts or groups",
			},
			"with": {
				Type: schema.Object,
				Desc: "Action parameters",
			},
			"summary": {
				Type: schema.String,
				Desc: "Short summary of the change",
			},
		}),
	}, nil
}

func (t *stepPatchTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	if t == nil || t.pipeline == nil || t.state == nil || t.store == nil {
		return "", fmt.Errorf("step_patch tool is not ready")
	}
	patch, err := parseStepPatchJSON(argumentsInJSON)
	if err != nil {
		emitCustomEvent(t.state, "tool_done", "error", err.Error(), map[string]any{
			"tool": "step_patch",
		})
		return "", err
	}
	patch = alignStepPatchWithPlan(t.store, t.draftID, patch)
	emitCustomEvent(t.state, "tool_start", "start", "step_patch start", map[string]any{
		"tool":      "step_patch",
		"step_id":   patch.StepID,
		"step_name": patch.StepName,
	})
	patch.Source = "coder"
	t.store.UpdateStep(t.draftID, patch)
	var yamlText string
	if snapshot := t.store.Snapshot(t.draftID); snapshot.DraftID != "" {
		if next, err := buildFinalYAML(snapshot); err == nil {
			yamlText = next
		}
	}
	emitStepEvent(t.state, "plan_step_start", PlanStep{
		ID:       patch.StepID,
		StepName: patch.StepName,
	}, "start", "plan step", nil)
	emitStepEvent(t.state, "step_patch_created", PlanStep{
		ID:       patch.StepID,
		StepName: patch.StepName,
	}, "done", patch.Summary, map[string]any{
		"step_patch": patch,
		"yaml":       yamlText,
	})

	reviewResult := t.pipeline.reviewStep(ctx, t.state, t.store, t.draftID, ReviewTask{
		StepID: patch.StepID,
		Patch:  patch,
		Status: "pending",
	}, t.opts)

	status := StepStatusDone
	if reviewResult.Status == StepStatusFailed {
		status = StepStatusFailed
	}
	emitStepEvent(t.state, "plan_step_done", PlanStep{
		ID:       patch.StepID,
		StepName: patch.StepName,
	}, string(status), reviewResult.Summary, nil)
	emitCustomEvent(t.state, "tool_done", "done", reviewResult.Summary, map[string]any{
		"tool":      "step_patch",
		"step_id":   patch.StepID,
		"step_name": patch.StepName,
	})

	executed := []planexecute.ExecutedStep{}
	if value, ok := adk.GetSessionValue(ctx, planexecute.ExecutedStepsSessionKey); ok {
		if casted, ok := value.([]planexecute.ExecutedStep); ok {
			executed = casted
		}
	}
	executed = append(executed, planexecute.ExecutedStep{
		Step:   patch.StepName,
		Result: reviewResult.Summary,
	})
	adk.AddSessionValue(ctx, planexecute.ExecutedStepsSessionKey, executed)

	if t.opts.PauseAfterStep {
		info := &pauseInfo{
			StepID:   patch.StepID,
			StepName: patch.StepName,
			Reason:   "paused_after_step",
		}
		logging.L().Info("step_patch tool interrupt",
			zap.String("draft_id", t.draftID),
			zap.String("step_id", patch.StepID),
		)
		return "", compose.StatefulInterrupt(ctx, "paused_after_step", info)
	}

	resp := map[string]any{
		"step_id":   patch.StepID,
		"step_name": patch.StepName,
		"status":    string(reviewResult.Status),
		"summary":   strings.TrimSpace(reviewResult.Summary),
	}
	payload, _ := json.Marshal(resp)
	return string(payload), nil
}

var _ tool.InvokableTool = (*stepPatchTool)(nil)
