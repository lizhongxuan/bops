package runmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bops/internal/core"
	"bops/internal/eventbus"
	"bops/internal/logging"
	"bops/internal/state"
	"bops/internal/workflow"
	"go.uber.org/zap"
)

type Manager struct {
	store  state.Store
	bus    *eventbus.Bus
	mu     sync.Mutex
	active map[string]*RunContext
}

type RunContext struct {
	ID     string
	Cancel context.CancelFunc
}

func New(store state.Store) *Manager {
	return NewWithBus(store, nil)
}

func NewWithBus(store state.Store, bus *eventbus.Bus) *Manager {
	return &Manager{
		store:  store,
		bus:    bus,
		active: make(map[string]*RunContext),
	}
}

func (m *Manager) StartRun(ctx context.Context, wf workflow.Workflow) (string, context.Context, error) {
	runID := fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	runCtx, cancel := context.WithCancel(ctx)
	logging.L().Debug("run start", zap.String("run_id", runID), zap.String("workflow", wf.Name))

	run := state.RunState{
		RunID:           runID,
		WorkflowName:    wf.Name,
		WorkflowVersion: wf.Version,
		Status:          "running",
		StartedAt:       time.Now().UTC(),
		Steps:           []state.StepState{},
		Resources:       map[string]state.ResourceState{},
	}

	if err := m.appendRun(run); err != nil {
		cancel()
		logging.L().Debug("run append failed", zap.String("run_id", runID), zap.Error(err))
		return "", nil, err
	}

	m.publish(runID, wf.Name, core.EventWorkflowStart, core.EventInfo, map[string]any{
		"status": "running",
	})

	m.mu.Lock()
	m.active[runID] = &RunContext{ID: runID, Cancel: cancel}
	m.mu.Unlock()

	return runID, runCtx, nil
}

func (m *Manager) FinishRun(runID string, runErr error) error {
	logging.L().Debug("run finish", zap.String("run_id", runID), zap.Error(runErr))
	m.mu.Lock()
	if ctx, ok := m.active[runID]; ok {
		delete(m.active, runID)
		ctx.Cancel()
	}
	m.mu.Unlock()

	err := m.updateRun(runID, func(run *state.RunState) {
		run.FinishedAt = time.Now().UTC()
		if runErr != nil {
			run.Status = "failed"
			run.Message = runErr.Error()
		} else {
			run.Status = "success"
		}
	})

	status := "success"
	level := core.EventInfo
	if runErr != nil {
		status = "failed"
		level = core.EventError
	}

	data := map[string]any{
		"status":  status,
		"message": errString(runErr),
	}
	if summary, ok := m.endSummary(runID); ok {
		if summary.Status != "" {
			data["status"] = summary.Status
		}
		data["total_steps"] = summary.TotalSteps
		data["success_steps"] = summary.SuccessSteps
		data["failed_steps"] = summary.FailedSteps
		data["duration_ms"] = summary.DurationMs
		if len(summary.Issues) > 0 {
			data["issues"] = summary.Issues
		}
		if summary.Message != "" && data["message"] == "" {
			data["message"] = summary.Message
		}
	}

	m.publish(runID, "", core.EventWorkflowEnd, level, data)

	return err
}

func (m *Manager) CancelRun(runID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, ok := m.active[runID]
	if !ok {
		return false
	}
	ctx.Cancel()
	return true
}

func (m *Manager) StopRun(runID string) error {
	logging.L().Debug("run stop", zap.String("run_id", runID))
	m.mu.Lock()
	if ctx, ok := m.active[runID]; ok {
		delete(m.active, runID)
		ctx.Cancel()
	}
	m.mu.Unlock()

	err := m.updateRun(runID, func(run *state.RunState) {
		run.FinishedAt = time.Now().UTC()
		run.Status = "stopped"
		run.Message = "stopped by user"
	})

	data := map[string]any{
		"status":  "stopped",
		"message": "stopped by user",
	}
	if summary, ok := m.endSummary(runID); ok {
		if summary.Status != "" {
			data["status"] = summary.Status
		}
		data["total_steps"] = summary.TotalSteps
		data["success_steps"] = summary.SuccessSteps
		data["failed_steps"] = summary.FailedSteps
		data["duration_ms"] = summary.DurationMs
		if len(summary.Issues) > 0 {
			data["issues"] = summary.Issues
		}
		if summary.Message != "" && data["message"] == "" {
			data["message"] = summary.Message
		}
	}

	m.publish(runID, "", core.EventWorkflowEnd, core.EventWarn, data)

	return err
}

func (m *Manager) ListRuns() ([]state.RunState, error) {
	if m.store == nil {
		return nil, fmt.Errorf("state store is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := m.store.Load()
	if err != nil {
		return nil, err
	}
	return data.Runs, nil
}

func (m *Manager) GetRun(runID string) (state.RunState, bool, error) {
	if m.store == nil {
		return state.RunState{}, false, fmt.Errorf("state store is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := m.store.Load()
	if err != nil {
		return state.RunState{}, false, err
	}
	for _, run := range data.Runs {
		if run.RunID == runID {
			return run, true, nil
		}
	}
	return state.RunState{}, false, nil
}

func (m *Manager) appendRun(run state.RunState) error {
	if m.store == nil {
		return fmt.Errorf("state store is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := m.store.Load()
	if err != nil {
		return err
	}
	data.Runs = append(data.Runs, run)
	return m.store.Save(data)
}

func (m *Manager) updateRun(runID string, apply func(*state.RunState)) error {
	if m.store == nil {
		return fmt.Errorf("state store is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := m.store.Load()
	if err != nil {
		return err
	}

	for i := range data.Runs {
		if data.Runs[i].RunID == runID {
			apply(&data.Runs[i])
			return m.store.Save(data)
		}
	}

	return fmt.Errorf("run %s not found", runID)
}

func (m *Manager) endSummary(runID string) (EndSummary, bool) {
	run, ok, err := m.GetRun(runID)
	if err != nil || !ok {
		return EndSummary{}, false
	}
	return BuildEndSummary(run), true
}

func (m *Manager) publish(runID, workflowName string, eventType core.EventType, level core.EventLevel, data map[string]any) {
	if m.bus == nil {
		return
	}
	m.bus.Publish(core.Event{
		Type:       eventType,
		Level:      level,
		Time:       time.Now().UTC(),
		RunID:      runID,
		WorkflowID: workflowName,
		Data:       data,
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
