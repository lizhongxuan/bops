package workbench

import (
	"strings"
	"testing"

	"bops/internal/workflow"
)

func TestGraphFromYAML(t *testing.T) {
	yamlText := strings.TrimSpace(`
version: v0.1
name: demo
steps:
  - name: step-a
    action: cmd.run
    with:
      cmd: echo hi
  - name: step-b
    action: cmd.run
    with:
      cmd: echo there
`)
	graph, err := GraphFromYAML(yamlText)
	if err != nil {
		t.Fatalf("GraphFromYAML: %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
	if graph.Nodes[0].Name != "step-a" {
		t.Fatalf("expected first node step-a, got %q", graph.Nodes[0].Name)
	}
	if graph.Edges[0].Source == "" || graph.Edges[0].Target == "" {
		t.Fatalf("expected edge source/target")
	}
}

func TestApplyGraphToYAMLOrdersSteps(t *testing.T) {
	baseYAML := strings.TrimSpace(`
version: v0.1
name: demo
steps:
  - name: first
    action: cmd.run
    with:
      cmd: echo first
  - name: second
    action: cmd.run
    with:
      cmd: echo second
`)
	graph := Graph{
		Version: "v1",
		Nodes: []Node{
			{ID: "node-b", Name: "second", Action: "cmd.run"},
			{ID: "node-a", Name: "first", Action: "cmd.run"},
		},
		Edges: []Edge{
			{ID: "edge-1", Source: "node-b", Target: "node-a"},
		},
	}
	updated, err := ApplyGraphToYAML(graph, baseYAML)
	if err != nil {
		t.Fatalf("ApplyGraphToYAML: %v", err)
	}
	wf, err := workflow.Load([]byte(updated))
	if err != nil {
		t.Fatalf("load updated yaml: %v", err)
	}
	if len(wf.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(wf.Steps))
	}
	if wf.Steps[0].Name != "second" || wf.Steps[1].Name != "first" {
		t.Fatalf("unexpected order: %+v", []string{wf.Steps[0].Name, wf.Steps[1].Name})
	}
}
