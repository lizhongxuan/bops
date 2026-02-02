package aiworkflow

import "testing"

func TestParsePlanJSONArray(t *testing.T) {
	payload := `[{"step_name":"install","description":"install nginx","dependencies":[]}]`
	steps, err := parsePlanJSON(payload)
	if err != nil {
		t.Fatalf("parsePlanJSON error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].StepName != "install" {
		t.Fatalf("unexpected step name: %s", steps[0].StepName)
	}
}

func TestParsePlanJSONObject(t *testing.T) {
	payload := `{"steps":[{"step_name":"config","description":"render config","dependencies":["install"]}]}`
	steps, err := parsePlanJSON(payload)
	if err != nil {
		t.Fatalf("parsePlanJSON error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Dependencies[0] != "install" {
		t.Fatalf("unexpected dependencies: %v", steps[0].Dependencies)
	}
}
