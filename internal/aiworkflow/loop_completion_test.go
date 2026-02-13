package aiworkflow

import "testing"

func TestEvaluateLoopCompletionTokenAndChecks(t *testing.T) {
	state := &State{
		YAML: `version: v0.1
name: demo
steps:
  - name: step1
    action: cmd.run
    args:
      cmd: echo hi`,
	}
	snapshot := defaultLoopMemorySnapshot("s")
	snapshot.PRD.UserStories = []LoopPRDStory{{ID: "US-1", Passes: true}}
	snapshot.Checkpoint.ToolHistory = []string{"tool=test output=PASS"}

	eval := evaluateLoopCompletion(state, loopAction{
		Action:  "final",
		Message: "<promise>COMPLETE</promise>",
	}, "", RunOptions{
		CompletionToken:  "<promise>COMPLETE</promise>",
		CompletionChecks: []string{"has_steps", "tests_green", "prd_all_pass"},
	}, snapshot)
	if !eval.Passed {
		t.Fatalf("expected completion checks to pass, got %+v", eval)
	}
}

func TestEvaluateLoopCompletionUnknownCheckFails(t *testing.T) {
	eval := evaluateLoopCompletion(&State{}, loopAction{Action: "final"}, "", RunOptions{
		CompletionChecks: []string{"not_exist"},
	}, defaultLoopMemorySnapshot("s"))
	if eval.Passed {
		t.Fatalf("expected unknown check to fail")
	}
	if len(eval.Failed) == 0 {
		t.Fatalf("expected failure reasons")
	}
}
