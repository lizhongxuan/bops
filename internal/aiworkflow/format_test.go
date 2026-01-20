package aiworkflow

import (
	"strings"
	"testing"
)

func TestExtractWorkflowYAMLFromJSON(t *testing.T) {
	jsonReply := `{"version":"v0.1","name":"demo","steps":[{"name":"step","action":"cmd.run","targets":["local"],"with":{"cmd":"echo hi"}}]}`

	yamlText, err := extractWorkflowYAML(jsonReply)
	if err != nil {
		t.Fatalf("extract yaml: %v", err)
	}
	if !containsAll(yamlText, []string{"version: v0.1", "name: demo", "action: cmd.run"}) {
		t.Fatalf("unexpected yaml output: %s", yamlText)
	}
}

func TestExtractWorkflowYAMLFromCodeBlock(t *testing.T) {
	reply := "```yaml\nversion: v0.1\nname: demo\nsteps:\n  - name: step\n    action: cmd.run\n    targets: [local]\n```\n"
	yamlText, err := extractWorkflowYAML(reply)
	if err != nil {
		t.Fatalf("extract yaml: %v", err)
	}
	if !containsAll(yamlText, []string{"version: v0.1", "name: demo", "action: cmd.run"}) {
		t.Fatalf("unexpected yaml output: %s", yamlText)
	}
}

func containsAll(text string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
