package engine

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"bops/runner/executor"
	"bops/runner/logging"
	"bops/runner/modules"
	"bops/runner/planner"
	"bops/runner/scheduler"
	"bops/runner/state"
	"bops/runner/workflow"
	"go.uber.org/zap"
)

type Engine struct {
	Registry         *modules.Registry
	Dispatcher       scheduler.Dispatcher
	RunStore         state.RunStateStore
	Notifier         state.RunStateNotifier
	NotifyRetry      int
	NotifyDelay      time.Duration
	Verbose          bool
	Out              io.Writer
	fallbackWarnOnce sync.Once
}

func New(registry *modules.Registry) *Engine {
	return &Engine{
		Registry:   registry,
		Dispatcher: scheduler.NewLocalDispatcher(registry),
		RunStore:   state.NewInMemoryRunStore(),
	}
}

func (e *Engine) Plan(ctx context.Context, wf workflow.Workflow) (planner.Plan, error) {
	if e.Registry == nil {
		return planner.Plan{}, fmt.Errorf("registry is nil")
	}
	logging.L().Debug("engine plan start",
		zap.String("workflow", wf.Name),
		zap.Int("steps", len(wf.Steps)),
	)

	hosts := wf.Inventory.ResolveHosts()
	plan := planner.Plan{
		ID:           fmt.Sprintf("plan-%d", time.Now().UTC().UnixNano()),
		WorkflowName: wf.Name,
		CreatedAt:    time.Now().UTC(),
	}

	for _, step := range wf.Steps {
		shouldRun, err := evalWhen(step.When, wf.Vars)
		if err != nil {
			logging.L().Debug("engine plan eval when failed",
				zap.String("step", step.Name),
				zap.Error(err),
			)
			return planner.Plan{}, err
		}
		if !shouldRun {
			continue
		}

		targets, err := resolveTargets(step, hosts, wf.Inventory)
		if err != nil {
			logging.L().Debug("engine plan resolve targets failed",
				zap.String("step", step.Name),
				zap.Error(err),
			)
			return planner.Plan{}, err
		}

		loopItems := step.Loop
		if len(loopItems) == 0 {
			loopItems = []any{nil}
		}

		stepPlan := planner.StepPlan{
			Name:    step.Name,
			Action:  step.Action,
			Targets: targetNames(targets),
		}

		for _, item := range loopItems {
			for _, target := range targets {
				module, ok := e.Registry.Get(step.Action)
				if !ok {
					return planner.Plan{}, fmt.Errorf("module %q not registered", step.Action)
				}

				vars := mergeVars(target.Vars, wf.Vars)
				if item != nil {
					vars = mergeVars(vars, map[string]any{"item": item})
				}

				res, err := module.Check(ctx, modules.Request{
					Step: step,
					Host: target,
					Vars: vars,
				})
				if err != nil {
					logging.L().Debug("engine plan module check failed",
						zap.String("step", step.Name),
						zap.String("host", target.Name),
						zap.Error(err),
					)
					return planner.Plan{}, err
				}
				if res.Changed {
					stepPlan.Changes = append(stepPlan.Changes, planner.ResourceChange{
						ResourceID: fmt.Sprintf("%s:%s", step.Name, target.Name),
						Diff:       wrapDiff(res.Diff),
					})
				}
			}
		}

		plan.Steps = append(plan.Steps, stepPlan)
	}

	logging.L().Debug("engine plan done",
		zap.String("workflow", wf.Name),
		zap.Int("steps", len(plan.Steps)),
	)
	return plan, nil
}

func (e *Engine) Apply(ctx context.Context, wf workflow.Workflow) error {
	_, err := e.ApplyWithRun(ctx, wf, RunOptions{})
	return err
}

func (e *Engine) ApplyWithRun(ctx context.Context, wf workflow.Workflow, opts RunOptions) (state.RunState, error) {
	logging.L().Debug("engine apply start",
		zap.String("workflow", wf.Name),
		zap.Int("steps", len(wf.Steps)),
	)
	store := opts.Store
	if store == nil {
		store = e.RunStore
	}
	if store == nil {
		store = state.NewInMemoryRunStore()
	}
	if isInMemoryStore(store) {
		e.fallbackWarnOnce.Do(func() {
			logging.L().Warn("run state store is in-memory only (non-durable); inject a durable RunStateStore for production")
		})
	}
	notifier := opts.Notifier
	if notifier == nil {
		notifier = e.Notifier
	}
	if opts.NotifyRetry == 0 {
		opts.NotifyRetry = e.NotifyRetry
	}
	if opts.NotifyDelay <= 0 {
		if e.NotifyDelay > 0 {
			opts.NotifyDelay = e.NotifyDelay
		} else {
			opts.NotifyDelay = 300 * time.Millisecond
		}
	}

	tracker, err := newRunTracker(wf, RunOptions{
		RunID:       opts.RunID,
		Store:       store,
		Notifier:    notifier,
		NotifyRetry: opts.NotifyRetry,
		NotifyDelay: opts.NotifyDelay,
	}, store)
	if err != nil {
		return state.RunState{}, err
	}
	if err := tracker.Start(ctx); err != nil {
		return state.RunState{}, err
	}

	baseRecorder := recorderFromContext(ctx)
	recorder := MultiRecorder(baseRecorder, tracker)
	env := envFromContext(ctx)
	runner := &dispatchRunner{
		dispatcher: e.Dispatcher,
		verbose:    e.Verbose,
		out:        e.Out,
		recorder:   recorder,
		env:        env,
		runID:      tracker.RunID(),
	}
	exec := &executor.Executor{
		Runner:   runner,
		Observer: recorder,
	}
	err = exec.Run(ctx, wf)
	if err != nil {
		status := state.RunStatusFailed
		if ctx.Err() != nil {
			status = state.RunStatusCanceled
		}
		if finishErr := tracker.Finish(ctx, status, err.Error(), err); finishErr != nil {
			return tracker.Snapshot(), fmt.Errorf("finalize run status: %w (execution error: %v)", finishErr, err)
		}
		logging.L().Debug("engine apply failed",
			zap.String("workflow", wf.Name),
			zap.String("run_id", tracker.RunID()),
			zap.Error(err),
		)
		return tracker.Snapshot(), err
	}
	if finishErr := tracker.Finish(ctx, state.RunStatusSuccess, "", nil); finishErr != nil {
		return tracker.Snapshot(), finishErr
	}
	logging.L().Debug("engine apply done",
		zap.String("workflow", wf.Name),
		zap.String("run_id", tracker.RunID()),
	)
	return tracker.Snapshot(), nil
}

func (e *Engine) ReconcileRunning(ctx context.Context, store state.RunStateStore, reason string) (int, error) {
	targetStore := store
	if targetStore == nil {
		targetStore = e.RunStore
	}
	if targetStore == nil {
		return 0, nil
	}
	updated, err := targetStore.MarkInterruptedRunning(ctx, reason)
	if err != nil {
		return 0, err
	}
	if updated > 0 {
		logging.L().Info("reconciled interrupted runs",
			zap.Int("count", updated),
			zap.String("reason", reason),
		)
	}
	return updated, nil
}

func isInMemoryStore(store state.RunStateStore) bool {
	if store == nil {
		return true
	}
	_, ok := store.(*state.InMemoryRunStore)
	return ok
}

type dispatchRunner struct {
	dispatcher scheduler.Dispatcher
	verbose    bool
	out        io.Writer
	recorder   Recorder
	mu         sync.Mutex
	env        map[string]string
	runID      string
}

func (r *dispatchRunner) Run(ctx context.Context, step workflow.Step, host workflow.HostSpec, vars map[string]any) (executor.RunResult, error) {
	if r.dispatcher == nil {
		return executor.RunResult{}, fmt.Errorf("dispatcher is nil")
	}
	logging.L().Debug("dispatch run",
		zap.String("run_id", r.runID),
		zap.String("step", step.Name),
		zap.String("action", step.Action),
		zap.String("host", host.Name),
	)
	taskVars := r.injectEnv(vars)
	taskID := fmt.Sprintf("task-%s-%s-%d", step.Name, host.Name, time.Now().UTC().UnixNano())
	if strings.TrimSpace(r.runID) != "" {
		taskID = fmt.Sprintf("%s-%s", r.runID, taskID)
	}
	result, err := r.dispatcher.Dispatch(ctx, scheduler.Task{
		ID:    taskID,
		RunID: r.runID,
		Step:  step,
		Host:  host,
		Vars:  taskVars,
	})
	if r.verbose {
		r.printResult(step, host, result)
	}
	if r.recorder != nil {
		r.recorder.HostResult(step, host, result)
	}
	if step.Action == "env.set" {
		r.mergeEnvFromOutput(result.Output)
	}
	if err != nil {
		logging.L().Debug("dispatch failed",
			zap.String("run_id", r.runID),
			zap.String("step", step.Name),
			zap.String("host", host.Name),
			zap.Error(err),
		)
		return executor.RunResult{Output: result.Output}, err
	}
	if result.Status != "success" {
		logging.L().Debug("dispatch result not success",
			zap.String("run_id", r.runID),
			zap.String("step", step.Name),
			zap.String("host", host.Name),
			zap.String("status", result.Status),
		)
		return executor.RunResult{Output: result.Output}, fmt.Errorf("task failed: %s", result.Error)
	}
	logging.L().Debug("dispatch done",
		zap.String("run_id", r.runID),
		zap.String("step", step.Name),
		zap.String("host", host.Name),
	)
	return executor.RunResult{Output: result.Output}, nil
}

func (r *dispatchRunner) injectEnv(vars map[string]any) map[string]any {
	if len(r.env) == 0 {
		return vars
	}

	envCopy := map[string]any{}
	r.mu.Lock()
	for k, v := range r.env {
		envCopy[k] = v
	}
	r.mu.Unlock()

	out := make(map[string]any, len(vars)+1)
	for k, v := range vars {
		out[k] = v
	}
	out["env"] = envCopy
	return out
}

func (r *dispatchRunner) mergeEnvFromOutput(output map[string]any) {
	if len(output) == 0 {
		return
	}
	raw, ok := output["env"]
	if !ok {
		return
	}

	parsed := map[string]string{}
	switch env := raw.(type) {
	case map[string]string:
		for k, v := range env {
			parsed[k] = v
		}
	case map[string]any:
		for k, v := range env {
			parsed[k] = fmt.Sprint(v)
		}
	case map[any]any:
		for k, v := range env {
			parsed[fmt.Sprint(k)] = fmt.Sprint(v)
		}
	default:
		return
	}

	if len(parsed) == 0 {
		return
	}

	r.mu.Lock()
	if r.env == nil {
		r.env = map[string]string{}
	}
	for k, v := range parsed {
		r.env[k] = v
	}
	r.mu.Unlock()
}

func (r *dispatchRunner) printResult(step workflow.Step, host workflow.HostSpec, result scheduler.Result) {
	if r.out == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	fmt.Fprintf(r.out, "step=%s host=%s status=%s\n", step.Name, host.Name, result.Status)

	if stdout := readOutputString(result.Output, "stdout"); stdout != "" {
		fmt.Fprintf(r.out, "stdout:\n%s\n", stdout)
	}
	if stderr := readOutputString(result.Output, "stderr"); stderr != "" {
		fmt.Fprintf(r.out, "stderr:\n%s\n", stderr)
	}
}

func readOutputString(output map[string]any, key string) string {
	if len(output) == 0 {
		return ""
	}
	value, ok := output[key]
	if !ok {
		return ""
	}
	raw := fmt.Sprint(value)
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return raw
}

func wrapDiff(diff map[string]any) map[string]planner.DiffEntry {
	out := make(map[string]planner.DiffEntry, len(diff))
	for k, v := range diff {
		out[k] = planner.DiffEntry{Current: nil, Desired: v}
	}
	return out
}

func targetNames(targets []workflow.HostSpec) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.Name)
	}
	return out
}

func evalWhen(expr string, vars map[string]any) (bool, error) {
	return workflow.EvalWhen(expr, vars)
}

func resolveTargets(step workflow.Step, hosts map[string]workflow.HostSpec, inv workflow.Inventory) ([]workflow.HostSpec, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts defined in inventory")
	}

	selected := map[string]workflow.HostSpec{}
	if len(step.Targets) == 0 {
		for name, host := range hosts {
			selected[name] = host
		}
		return stableHosts(selected), nil
	}

	for _, target := range step.Targets {
		if host, ok := hosts[target]; ok {
			selected[target] = host
			continue
		}
		if group, ok := inv.Groups[target]; ok {
			for _, hostName := range group.Hosts {
				if host, ok := hosts[hostName]; ok {
					selected[hostName] = host
				}
			}
			continue
		}
		return nil, fmt.Errorf("unknown target %q", target)
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no targets resolved for step %q", step.Name)
	}

	return stableHosts(selected), nil
}

func stableHosts(hosts map[string]workflow.HostSpec) []workflow.HostSpec {
	names := make([]string, 0, len(hosts))
	for name := range hosts {
		names = append(names, name)
	}
	sort.Strings(names)

	ordered := make([]workflow.HostSpec, 0, len(names))
	for _, name := range names {
		ordered = append(ordered, hosts[name])
	}
	return ordered
}

func mergeVars(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
