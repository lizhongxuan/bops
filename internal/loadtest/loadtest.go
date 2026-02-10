package loadtest

import (
	"context"
	"fmt"
	"time"

	"bops/runner/engine"
	"bops/runner/workflow"
)

func GenerateWorkflow(stepCount, hostCount int) workflow.Workflow {
	hosts := map[string]workflow.Host{}
	for i := 0; i < hostCount; i++ {
		name := fmt.Sprintf("host-%d", i)
		hosts[name] = workflow.Host{Address: "127.0.0.1"}
	}

	steps := make([]workflow.Step, 0, stepCount)
	for i := 0; i < stepCount; i++ {
		steps = append(steps, workflow.Step{
			Name:    fmt.Sprintf("step-%d", i),
			Targets: []string{},
			Action:  "cmd.run",
			Args: map[string]any{
				"cmd": "echo loadtest",
			},
		})
	}

	return workflow.Workflow{
		Version: "v0.1",
		Name:    "loadtest",
		Inventory: workflow.Inventory{
			Hosts: hosts,
		},
		Plan:  workflow.Plan{Mode: "auto", Strategy: "sequential"},
		Steps: steps,
	}
}

func Run(ctx context.Context, eng *engine.Engine, wf workflow.Workflow) (time.Duration, error) {
	start := time.Now()
	if err := eng.Apply(ctx, wf); err != nil {
		return time.Since(start), err
	}
	return time.Since(start), nil
}
