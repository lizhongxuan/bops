package aiworkflow

import "testing"

func TestDraftStoreUpdate(t *testing.T) {
	store := NewDraftStore()
	draft := store.GetOrCreate("draft-1", "")
	if draft == nil || draft.DraftID != "draft-1" {
		t.Fatalf("expected draft created")
	}
	plan := []PlanStep{{ID: "step-1", StepName: "install"}}
	store.UpdatePlan("draft-1", plan)
	store.UpdateStep("draft-1", StepPatch{StepID: "step-1", StepName: "install", Action: "cmd.run", With: map[string]any{"cmd": "echo hi"}})
	store.UpdateReview("draft-1", ReviewResult{StepID: "step-1", Status: StepStatusDone})
	snapshot := store.Snapshot("draft-1")
	if len(snapshot.Plan) != 1 {
		t.Fatalf("expected plan in snapshot")
	}
	if snapshot.Steps["step-1"].Action != "cmd.run" {
		t.Fatalf("expected step patch in snapshot")
	}
	if snapshot.Reviews["step-1"].Status != StepStatusDone {
		t.Fatalf("expected review status in snapshot")
	}
}
