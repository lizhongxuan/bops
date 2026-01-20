package aiworkflow

import "testing"

func TestEvaluateRiskWithAllowlist(t *testing.T) {
	rules := DefaultRiskRules()

	level, notes := EvaluateRisk("rm -rf /tmp/cache\n", rules)
	if level != RiskLevelLow {
		t.Fatalf("expected low risk, got %s", level)
	}
	if len(notes) != 0 {
		t.Fatalf("expected no notes, got %v", notes)
	}

	level, notes = EvaluateRisk("rm -rf /\n", rules)
	if level != RiskLevelHigh {
		t.Fatalf("expected high risk, got %s", level)
	}
	if len(notes) == 0 {
		t.Fatalf("expected notes for high risk")
	}
}
