package aiworkflow

import (
	"context"
	"strings"
	"time"

	"bops/runner/logging"
	"go.uber.org/zap"
)

func normalizeAgentSpec(spec AgentSpec) AgentSpec {
	if spec.Name == "" {
		spec.Name = "main"
	}
	if spec.Role == "" {
		spec.Role = "primary"
	}
	return spec
}

func wrapEventSinkWithAgent(sink EventSink, spec AgentSpec) EventSink {
	if sink == nil {
		return nil
	}
	return func(evt Event) {
		if evt.AgentID == "" {
			evt.AgentID = spec.Name
		}
		if evt.AgentName == "" {
			evt.AgentName = spec.Name
		}
		if evt.AgentRole == "" {
			evt.AgentRole = spec.Role
		}
		sink(evt)
	}
}

func applyAgentState(state *State, spec AgentSpec) {
	if state == nil {
		return
	}
	state.AgentID = spec.Name
	state.AgentName = spec.Name
	state.AgentRole = spec.Role
}

func setStepStatus(state *State, stepID string, status StepStatus) {
	if state == nil || strings.TrimSpace(stepID) == "" {
		return
	}
	if state.StepStatuses == nil {
		state.StepStatuses = make(map[string]StepStatus)
	}
	state.StepStatuses[stepID] = status
}

func (p *Pipeline) RunAgent(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	spec := normalizeAgentSpec(opts.AgentSpec)
	opts.EventSink = wrapEventSinkWithAgent(opts.EventSink, spec)
	started := time.Now()
	logging.L().Info("agent start",
		zap.String("agent", spec.Name),
		zap.String("role", spec.Role),
		zap.String("mode", "generate"),
		zap.Int("prompt_len", len(prompt)),
	)
	store := NewStateStore(opts.BaseYAML)
	state, err := p.RunGenerate(ctx, prompt, context, opts)
	applyAgentState(state, spec)
	if state != nil {
		if len(state.Plan) > 0 {
			store.SetPlan(state.Plan)
		}
		if state.Context != nil {
			store.SetVars(state.Context)
		}
		if state.YAML != "" {
			_ = store.UpdateYAMLFragment(state.YAML, "")
		}
		snapshot := store.Snapshot()
		state.History = snapshot.History
	}
	if err != nil {
		logging.L().Error("agent end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.String("mode", "generate"),
			zap.Error(err),
			zap.Duration("elapsed", time.Since(started)),
		)
	} else {
		logging.L().Info("agent end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.String("mode", "generate"),
			zap.Duration("elapsed", time.Since(started)),
		)
	}
	return state, err
}

func (p *Pipeline) RunAgentFix(ctx context.Context, yaml string, issues []string, opts RunOptions) (*State, error) {
	spec := normalizeAgentSpec(opts.AgentSpec)
	opts.EventSink = wrapEventSinkWithAgent(opts.EventSink, spec)
	started := time.Now()
	logging.L().Info("agent start",
		zap.String("agent", spec.Name),
		zap.String("role", spec.Role),
		zap.String("mode", "fix"),
		zap.Int("yaml_len", len(yaml)),
		zap.Int("issues", len(issues)),
	)
	state, err := p.RunFix(ctx, yaml, issues, opts)
	applyAgentState(state, spec)
	if err != nil {
		logging.L().Error("agent end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.String("mode", "fix"),
			zap.Error(err),
			zap.Duration("elapsed", time.Since(started)),
		)
	} else {
		logging.L().Info("agent end",
			zap.String("agent", spec.Name),
			zap.String("role", spec.Role),
			zap.String("mode", "fix"),
			zap.Duration("elapsed", time.Since(started)),
		)
	}
	return state, err
}
