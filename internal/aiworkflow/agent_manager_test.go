package aiworkflow

import "testing"

func TestAgentManagerRegisterGetList(t *testing.T) {
	mgr := NewAgentManager()
	if _, ok := mgr.Get("main"); !ok {
		t.Fatalf("expected default main agent")
	}
	spec := AgentSpec{Name: "reviewer", Role: "qa", Skills: []string{"lint"}}
	if err := mgr.Register(spec); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	got, ok := mgr.Get("reviewer")
	if !ok {
		t.Fatalf("expected reviewer agent")
	}
	if got.Role != "qa" || len(got.Skills) != 1 {
		t.Fatalf("unexpected agent spec: %+v", got)
	}
	list := mgr.List()
	if len(list) < 2 {
		t.Fatalf("expected list to include default and reviewer")
	}
}
