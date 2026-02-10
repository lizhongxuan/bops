package aiworkflow

import (
	"context"
	"testing"
)

func TestReviewWorkerUpdatesStore(t *testing.T) {
	store := NewDraftStore()
	store.GetOrCreate("draft-1", "")
	p := &Pipeline{}
	state := &State{EventSink: func(Event) {}}
	tasks := make(chan ReviewTask, 1)
	tasks <- ReviewTask{
		StepID: "step-1",
		Patch: StepPatch{
			StepID:   "step-1",
			StepName: "install nginx",
			Action:   "cmd.run",
			Args:     map[string]any{"cmd": "echo hi"},
			Summary:  "install nginx",
		},
		Attempt: 0,
		Status:  "pending",
	}
	close(tasks)
	p.reviewWorker(context.Background(), state, store, "draft-1", tasks, RunOptions{SkipExecute: true})
	snapshot := store.Snapshot("draft-1")
	result, ok := snapshot.Reviews["step-1"]
	if !ok {
		t.Fatalf("expected review result")
	}
	if result.Status != StepStatusDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}
	if snapshot.Steps["step-1"].Action != "cmd.run" {
		t.Fatalf("expected step to be stored")
	}
}
