package aiworkflow

import "context"

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

func (p *Pipeline) RunAgent(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	spec := normalizeAgentSpec(opts.AgentSpec)
	opts.EventSink = wrapEventSinkWithAgent(opts.EventSink, spec)
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
			_ = store.UpdateYAMLFragment(state.YAML)
		}
		snapshot := store.Snapshot()
		state.History = snapshot.History
	}
	return state, err
}

func (p *Pipeline) RunAgentFix(ctx context.Context, yaml string, issues []string, opts RunOptions) (*State, error) {
	spec := normalizeAgentSpec(opts.AgentSpec)
	opts.EventSink = wrapEventSinkWithAgent(opts.EventSink, spec)
	state, err := p.RunFix(ctx, yaml, issues, opts)
	applyAgentState(state, spec)
	return state, err
}
