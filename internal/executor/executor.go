package executor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"bops/internal/workflow"
)

type HostRunner interface {
	Run(ctx context.Context, step workflow.Step, host workflow.HostSpec, vars map[string]any) error
}

type Observer interface {
	StepStart(step workflow.Step, targets []workflow.HostSpec)
	StepFinish(step workflow.Step, status string)
}

type Executor struct {
	Runner   HostRunner
	Observer Observer
}

func (e *Executor) Run(ctx context.Context, wf workflow.Workflow) error {
	if e.Runner == nil {
		return fmt.Errorf("executor runner is nil")
	}

	hosts := wf.Inventory.ResolveHosts()
	handlers := map[string]workflow.Handler{}
	for _, handler := range wf.Handlers {
		handlers[handler.Name] = handler
	}

	for _, step := range wf.Steps {
		shouldRun, err := evalWhen(step.When)
		if err != nil {
			return err
		}
		if !shouldRun {
			continue
		}

		targets, err := resolveTargets(step, hosts, wf.Inventory)
		if err != nil {
			return err
		}

		if e.Observer != nil {
			e.Observer.StepStart(step, targets)
		}

		loopItems := step.Loop
		if len(loopItems) == 0 {
			loopItems = []any{nil}
		}

		for _, item := range loopItems {
			if err := e.runOnTargets(ctx, step, targets, wf.Vars, item); err != nil {
				if e.Observer != nil {
					e.Observer.StepFinish(step, "failed")
				}
				return err
			}
			if len(step.Notify) > 0 {
				if err := e.runHandlers(ctx, handlers, step.Notify, targets, wf.Vars, item); err != nil {
					if e.Observer != nil {
						e.Observer.StepFinish(step, "failed")
					}
					return err
				}
			}
		}

		if e.Observer != nil {
			e.Observer.StepFinish(step, "success")
		}
	}

	return nil
}

func (e *Executor) runOnTargets(ctx context.Context, step workflow.Step, targets []workflow.HostSpec, baseVars map[string]any, item any) error {
	timeout, err := parseTimeout(step.Timeout)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(targets))

	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()

			vars := mergeVars(target.Vars, baseVars)
			if item != nil {
				vars = mergeVars(vars, map[string]any{"item": item})
			}

			if err := runWithRetry(ctx, e.Runner, step, target, vars, timeout); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Executor) runHandlers(ctx context.Context, handlers map[string]workflow.Handler, notify []string, targets []workflow.HostSpec, baseVars map[string]any, item any) error {
	seen := map[string]struct{}{}
	for _, name := range notify {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		handler, ok := handlers[name]
		if !ok {
			return fmt.Errorf("handler %q not found", name)
		}

		shouldRun, err := evalWhen(handler.When)
		if err != nil {
			return err
		}
		if !shouldRun {
			continue
		}

		step := workflow.Step{
			Name:    handler.Name,
			Action:  handler.Action,
			With:    handler.With,
			Retries: handler.Retries,
			Timeout: handler.Timeout,
		}

		if err := e.runOnTargets(ctx, step, targets, baseVars, item); err != nil {
			return err
		}
	}
	return nil
}

func runWithRetry(ctx context.Context, runner HostRunner, step workflow.Step, host workflow.HostSpec, vars map[string]any, timeout time.Duration) error {
	attempts := step.Retries + 1
	var lastErr error

	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		runCtx := ctx
		cancel := func() {}
		if timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, timeout)
		}

		err := runner.Run(runCtx, step, host, vars)
		cancel()

		if err == nil {
			return nil
		}
		lastErr = err
	}

	return lastErr
}

func parseTimeout(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q", raw)
	}
	return parsed, nil
}

func evalWhen(expr string) (bool, error) {
	trimmed := strings.TrimSpace(strings.ToLower(expr))
	if trimmed == "" || trimmed == "true" || trimmed == "yes" {
		return true, nil
	}
	if trimmed == "false" || trimmed == "no" {
		return false, nil
	}
	return false, fmt.Errorf("unsupported when expression %q", expr)
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
