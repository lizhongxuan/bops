package aiworkflow

import "strings"

import "testing"

func TestStateStoreUpdateYAMLFragment(t *testing.T) {
	base := `version: v0.1
name: demo
description: ""
inventory:
  hosts:
    local:
      address: 127.0.0.1
plan:
  mode: manual-approve
  strategy: sequential
steps:
  - name: stepA
    action: cmd.run
    with:
      cmd: echo a
`
	fragment := `version: v0.1
name: demo
description: ""
inventory:
  hosts:
    local:
      address: 127.0.0.1
plan:
  mode: manual-approve
  strategy: sequential
steps:
  - name: stepB
    action: cmd.run
    with:
      cmd: echo b
`
	store := NewStateStore(base)
	if err := store.UpdateYAMLFragment(fragment, ""); err != nil {
		t.Fatalf("update fragment: %v", err)
	}
	snap := store.Snapshot()
	if !strings.Contains(snap.YAML, "stepB") {
		t.Fatalf("expected updated yaml to include stepB")
	}
	if strings.Contains(snap.YAML, "stepA") {
		t.Fatalf("expected updated yaml to replace stepA")
	}
	if len(snap.History) != 1 {
		t.Fatalf("expected history length 1, got %d", len(snap.History))
	}
}
