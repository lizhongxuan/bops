package aiworkflow

import (
	"strings"
	"testing"

	"bops/runner/workflow"
)

func TestConvertScriptToYAML(t *testing.T) {
	script := "echo hello"
	yamlText, err := ConvertScriptToYAML(script)
	if err != nil {
		t.Fatalf("convert script: %v", err)
	}
	if strings.TrimSpace(yamlText) == "" {
		t.Fatal("expected yaml output")
	}
	wf, err := workflow.Load([]byte(yamlText))
	if err != nil {
		t.Fatalf("load yaml: %v", err)
	}
	if len(wf.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(wf.Steps))
	}
	if wf.Steps[0].Action != "cmd.run" {
		t.Fatalf("unexpected action: %s", wf.Steps[0].Action)
	}
	if cmd, ok := wf.Steps[0].With["cmd"].(string); !ok || strings.TrimSpace(cmd) != script {
		t.Fatalf("unexpected cmd: %v", wf.Steps[0].With["cmd"])
	}
}
