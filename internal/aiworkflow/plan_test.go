package aiworkflow

import "testing"

func TestParsePlanJSONList(t *testing.T) {
	reply := `[{"step_name":"install nginx","description":"install package","dependencies":[]}]`
	steps, err := parsePlanJSON(reply)
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].StepName != "install nginx" {
		t.Fatalf("unexpected step_name: %q", steps[0].StepName)
	}
	if steps[0].ID == "" {
		t.Fatalf("expected step id to be set")
	}
	if steps[0].Status != StepStatusPending {
		t.Fatalf("expected pending status, got %q", steps[0].Status)
	}
}
