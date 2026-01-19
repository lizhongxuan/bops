package scheduler

import (
	"context"
	"fmt"

	"bops/internal/modules"
	"bops/internal/workflow"
)

type Task struct {
	ID   string
	Step workflow.Step
	Host workflow.HostSpec
	Vars map[string]any
}

type Result struct {
	TaskID string         `json:"task_id"`
	Status string         `json:"status"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type Dispatcher interface {
	Dispatch(ctx context.Context, task Task) (Result, error)
}

type LocalDispatcher struct {
	Registry *modules.Registry
}

func NewLocalDispatcher(registry *modules.Registry) *LocalDispatcher {
	return &LocalDispatcher{Registry: registry}
}

func (d *LocalDispatcher) Dispatch(ctx context.Context, task Task) (Result, error) {
	if d.Registry == nil {
		return Result{}, fmt.Errorf("registry is nil")
	}
	module, ok := d.Registry.Get(task.Step.Action)
	if !ok {
		return Result{}, fmt.Errorf("module %q not registered", task.Step.Action)
	}

	res, err := module.Apply(ctx, modules.Request{
		Step: task.Step,
		Host: task.Host,
		Vars: task.Vars,
	})
	if err != nil {
		return Result{
			TaskID: task.ID,
			Status: "failed",
			Output: res.Output,
			Error:  err.Error(),
		}, err
	}

	return Result{
		TaskID: task.ID,
		Status: "success",
		Output: res.Output,
	}, nil
}
