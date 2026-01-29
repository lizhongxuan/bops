package aiworkflow

import (
	"strings"
	"testing"

	"bops/internal/workflow"
)

func TestParseNodeRegenResponseEnvelope(t *testing.T) {
	resp := `{"step":{"name":"install nginx","action":"pkg.install","with":{"name":"nginx"}},"questions":["Which hosts?"]}`
	out, err := ParseNodeRegenResponse(resp)
	if err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	if strings.TrimSpace(out.Step.Name) != "install nginx" {
		t.Fatalf("expected step name, got %q", out.Step.Name)
	}
	if strings.TrimSpace(out.Step.Action) != "pkg.install" {
		t.Fatalf("expected step action, got %q", out.Step.Action)
	}
	if len(out.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(out.Questions))
	}
}

func TestParseNodeRegenResponseRawStep(t *testing.T) {
	resp := `{"name":"render config","action":"template.render","with":{"src":"a","dest":"b"}}`
	out, err := ParseNodeRegenResponse(resp)
	if err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	if strings.TrimSpace(out.Step.Action) != "template.render" {
		t.Fatalf("expected template.render, got %q", out.Step.Action)
	}
}

func TestNormalizeNodeRegenStep(t *testing.T) {
	step, issues := NormalizeNodeRegenStep(stepWith("", ""))
	if len(issues) == 0 {
		t.Fatalf("expected issues for empty step")
	}
	step, issues = NormalizeNodeRegenStep(stepWith("ok", "cmd.run"))
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if step.Name != "ok" {
		t.Fatalf("expected name ok, got %q", step.Name)
	}
}

func stepWith(name, action string) workflow.Step {
	return workflow.Step{
		Name:   name,
		Action: action,
		With:   map[string]any{"cmd": "echo hi"},
	}
}
