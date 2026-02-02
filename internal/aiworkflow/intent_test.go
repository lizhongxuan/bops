package aiworkflow

import "testing"

func TestNormalizeIntentType(t *testing.T) {
	cases := map[string]IntentType{
		"explain":  IntentExplain,
		"audit":    IntentExplain,
		"debug":    IntentDebug,
		"fix":      IntentDebug,
		"optimize": IntentOptimize,
		"simulate": IntentSimulate,
		"dry_run":  IntentSimulate,
		"migrate":  IntentMigrate,
		"convert":  IntentMigrate,
		"unknown":  IntentExplain,
		"":         IntentExplain,
	}
	for raw, expected := range cases {
		if got := normalizeIntentType(IntentType(raw)); got != expected {
			t.Fatalf("normalizeIntentType(%q)=%s, want %s", raw, got, expected)
		}
	}
}

func TestParseIntentResponse(t *testing.T) {
	payload := `{"intent_type":"debug","goal":"fix","missing":["targets"]}`
	intent, err := parseIntentResponse(payload)
	if err != nil {
		t.Fatalf("parseIntentResponse error: %v", err)
	}
	if intent.Type != IntentDebug {
		t.Fatalf("expected intent debug, got %s", intent.Type)
	}
	if intent.Goal != "fix" {
		t.Fatalf("expected goal fix, got %s", intent.Goal)
	}
	if len(intent.Missing) != 1 || intent.Missing[0] != "targets" {
		t.Fatalf("unexpected missing: %v", intent.Missing)
	}
}
