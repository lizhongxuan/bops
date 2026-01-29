package aiworkflow

import (
	"strings"
	"testing"

	"bops/internal/workflow"
)

func TestDetectComplexSteps(t *testing.T) {
	wf := workflow.Workflow{
		Steps: []workflow.Step{
			{
				Name:   "simple",
				Action: "cmd.run",
				With: map[string]any{
					"cmd": "echo ok",
				},
			},
			{
				Name:   "complex",
				Action: "script.shell",
				With: map[string]any{
					"cmd":  "echo hi && echo there",
					"foo":  "bar",
					"baz":  "qux",
					"path": "/opt/app/configs/long-template-path.yaml",
				},
			},
		},
	}
	complex := detectComplexSteps(wf)
	if len(complex) != 1 {
		t.Fatalf("expected 1 complex step, got %d", len(complex))
	}
	if complex[0].Name != "complex" {
		t.Fatalf("expected complex step name, got %q", complex[0].Name)
	}
	if !strings.Contains(complex[0].Reason, "action type") {
		t.Fatalf("expected reason to mention action type, got %q", complex[0].Reason)
	}
}
