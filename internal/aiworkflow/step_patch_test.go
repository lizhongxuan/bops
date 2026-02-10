package aiworkflow

import "testing"

func TestParseStepPatchJSONValid(t *testing.T) {
	reply := `{"step_name":"install nginx","action":"cmd.run","args":{"cmd":"echo hi"}}`
	patch, err := parseStepPatchJSON(reply)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if patch.StepName != "install nginx" {
		t.Fatalf("unexpected step_name: %q", patch.StepName)
	}
	if patch.StepID == "" {
		t.Fatalf("expected step_id to be set")
	}
	if patch.Action != "cmd.run" {
		t.Fatalf("unexpected action: %q", patch.Action)
	}
	if patch.Summary == "" {
		t.Fatalf("expected summary to be set")
	}
}

func TestParseStepPatchJSONUnknownField(t *testing.T) {
	reply := `{"step_name":"install nginx","action":"cmd.run","foo":"bar"}`
	_, err := parseStepPatchJSON(reply)
	if err == nil {
		t.Fatalf("expected error for unknown field")
	}
}

func TestValidateStepPatch(t *testing.T) {
	issues := validateStepPatch(StepPatch{StepName: "x"})
	if len(issues) == 0 {
		t.Fatalf("expected issues for missing action")
	}
}
