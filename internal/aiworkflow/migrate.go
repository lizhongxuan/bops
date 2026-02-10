package aiworkflow

import (
	"errors"
	"strings"

	"bops/runner/workflow"
)

func ConvertScriptToYAML(scriptText string) (string, error) {
	trimmed := strings.TrimSpace(scriptText)
	if trimmed == "" {
		return "", errors.New("script is empty")
	}
	wf := workflow.Workflow{
		Version:     defaultWorkflowVersion,
		Name:        defaultWorkflowName,
		Description: "migrated from script",
		Inventory: workflow.Inventory{
			Hosts: map[string]workflow.Host{
				"local": {Address: "127.0.0.1"},
			},
		},
		Plan: workflow.Plan{
			Mode:     defaultPlanMode,
			Strategy: defaultPlanStrategy,
		},
		Steps: []workflow.Step{
			{
				Name:   "run-script",
				Action: "cmd.run",
				Args: map[string]any{
					"cmd": trimmed,
				},
			},
		},
	}
	wf = normalizeWorkflow(wf)
	return marshalWorkflowYAML(wf)
}
