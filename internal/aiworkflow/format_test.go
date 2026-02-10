package aiworkflow

import (
	"strings"
	"testing"
)

func TestExtractWorkflowYAMLFromJSON(t *testing.T) {
	jsonReply := `{"workflow":{"version":"v0.1","name":"demo","steps":[{"name":"step","action":"cmd.run","targets":["local"],"args":{"cmd":"echo hi"}}]},"questions":["Which hosts?"]}`

	yamlText, questions, err := extractWorkflowYAML(jsonReply)
	if err != nil {
		t.Fatalf("extract yaml: %v", err)
	}
	if !containsAll(yamlText, []string{"version: v0.1", "steps:", "action: cmd.run"}) {
		t.Fatalf("unexpected yaml output: %s", yamlText)
	}
	if len(questions) != 1 || questions[0] != "Which hosts?" {
		t.Fatalf("unexpected questions: %v", questions)
	}
}

func TestExtractWorkflowYAMLFromCodeBlock(t *testing.T) {
	reply := "```yaml\nversion: v0.1\nname: demo\nsteps:\n  - name: step\n    action: cmd.run\n    targets: [local]\n```\n"
	yamlText, questions, err := extractWorkflowYAML(reply)
	if err != nil {
		t.Fatalf("extract yaml: %v", err)
	}
	if !containsAll(yamlText, []string{"version: v0.1", "steps:", "action: cmd.run"}) {
		t.Fatalf("unexpected yaml output: %s", yamlText)
	}
	if len(questions) != 0 {
		t.Fatalf("unexpected questions: %v", questions)
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
