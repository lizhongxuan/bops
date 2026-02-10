package aiworkflow

import "testing"

func TestRunSimulation(t *testing.T) {
	yamlText := `
version: v0.1
name: demo
description: test
inventory:
  hosts:
    local:
      address: 127.0.0.1
plan:
  mode: manual-approve
  strategy: sequential
steps:
  - name: step-one
    action: cmd.run
    args:
      cmd: echo hello
  - name: step-two
    action: cmd.run
    args:
      cmd: echo world
`
	result, err := RunSimulation(yamlText, map[string]any{"env": "test"})
	if err != nil {
		t.Fatalf("run simulation: %v", err)
	}
	if result == nil || len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %+v", result)
	}
	if result.Steps[0].Name != "step-one" {
		t.Fatalf("unexpected step name: %s", result.Steps[0].Name)
	}
	if len(result.Hosts) != 1 || result.Hosts[0] != "local" {
		t.Fatalf("unexpected hosts: %v", result.Hosts)
	}
}
