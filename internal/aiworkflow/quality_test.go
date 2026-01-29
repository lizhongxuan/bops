package aiworkflow

import (
	"strings"
	"testing"
)

func TestApplyQualityGuardYAMLTruncateAndName(t *testing.T) {
	steps := make([]string, 0, maxWorkflowStepCount+2)
	for i := 0; i < maxWorkflowStepCount+2; i++ {
		steps = append(steps, "  - name: \"\"\n    action: cmd.run\n    with:\n      cmd: \"echo hi\"")
	}
	yamlText := "version: v0.1\nname: demo\nsteps:\n" + strings.Join(steps, "\n")
	out := applyQualityGuardYAML(yamlText)
	if out.YAML == "" {
		t.Fatalf("expected yaml output")
	}
	if !strings.Contains(out.YAML, "steps:") {
		t.Fatalf("expected steps in yaml")
	}
	if len(out.Notices) == 0 {
		t.Fatalf("expected notices")
	}
}

func TestApplyQualityGuardYAMLDedupe(t *testing.T) {
	yamlText := `version: v0.1
name: demo
steps:
  - name: one
    action: cmd.run
    with:
      cmd: "echo hi"
  - name: one
    action: cmd.run
    with:
      cmd: "echo hi"`
	out := applyQualityGuardYAML(yamlText)
	if out.YAML == "" {
		t.Fatalf("expected yaml output")
	}
	if !strings.Contains(out.YAML, "steps:") {
		t.Fatalf("expected steps in yaml")
	}
}
