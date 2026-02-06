package runmanager

import (
	"fmt"
	"time"

	"bops/internal/core"
	"bops/runner/scheduler"
	"bops/runner/state"
	"bops/runner/workflow"
)

type Recorder struct {
	manager *Manager
	runID   string
}

func (m *Manager) Recorder(runID string) *Recorder {
	return &Recorder{manager: m, runID: runID}
}

func (r *Recorder) StepStart(step workflow.Step, targets []workflow.HostSpec) {
	now := time.Now().UTC()
	_ = r.manager.updateRun(r.runID, func(run *state.RunState) {
		stepState := ensureStep(run, step.Name)
		if stepState.StartedAt.IsZero() {
			stepState.StartedAt = now
		}
		stepState.Status = "running"

		if stepState.Hosts == nil {
			stepState.Hosts = make(map[string]state.HostResult, len(targets))
		}
		for _, target := range targets {
			if _, ok := stepState.Hosts[target.Name]; ok {
				continue
			}
			stepState.Hosts[target.Name] = state.HostResult{
				Host:      target.Name,
				Status:    "running",
				StartedAt: now,
			}
		}
	})

	r.publish(core.Event{
		Type:  core.EventStepStart,
		Level: core.EventInfo,
		Time:  now,
		RunID: r.runID,
		Step:  step.Name,
		Data:  map[string]any{"targets": targets},
	})
}

func (r *Recorder) StepFinish(step workflow.Step, status string) {
	now := time.Now().UTC()
	_ = r.manager.updateRun(r.runID, func(run *state.RunState) {
		stepState := ensureStep(run, step.Name)
		stepState.Status = status
		stepState.FinishedAt = now
	})

	eventType := core.EventStepEnd
	level := core.EventInfo
	if status == "failed" {
		eventType = core.EventStepFailed
		level = core.EventError
	}
	r.publish(core.Event{
		Type:  eventType,
		Level: level,
		Time:  now,
		RunID: r.runID,
		Step:  step.Name,
		Data:  map[string]any{"status": status},
	})
}

func (r *Recorder) HostResult(step workflow.Step, host workflow.HostSpec, result scheduler.Result) {
	now := time.Now().UTC()
	_ = r.manager.updateRun(r.runID, func(run *state.RunState) {
		stepState := ensureStep(run, step.Name)
		if stepState.Hosts == nil {
			stepState.Hosts = map[string]state.HostResult{}
		}
		entry := stepState.Hosts[host.Name]
		if entry.StartedAt.IsZero() {
			entry.StartedAt = now
		}
		entry.Host = host.Name
		entry.Status = result.Status
		entry.Output = result.Output
		entry.Message = result.Error
		entry.FinishedAt = now
		stepState.Hosts[host.Name] = entry
	})

	r.publish(core.Event{
		Type:  core.EventAgentOutput,
		Level: core.EventInfo,
		Time:  now,
		RunID: r.runID,
		Step:  step.Name,
		Host:  host.Name,
		Data: map[string]any{
			"status": result.Status,
			"output": result.Output,
			"error":  result.Error,
		},
	})
}

func ensureStep(run *state.RunState, name string) *state.StepState {
	for i := range run.Steps {
		if run.Steps[i].Name == name {
			return &run.Steps[i]
		}
	}
	run.Steps = append(run.Steps, state.StepState{
		Name:  name,
		Hosts: map[string]state.HostResult{},
	})
	return &run.Steps[len(run.Steps)-1]
}

func (r *Recorder) publish(event core.Event) {
	if r.manager.bus == nil {
		return
	}
	event.ID = fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())
	r.manager.bus.Publish(event)
}
