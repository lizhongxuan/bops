package workbench

import (
	"context"
	"testing"
)

func TestGraphExecutor_Run(t *testing.T) {
	graph := Graph{
		Version: "v1",
		Nodes: []Node{
			{
				ID:   "start",
				Type: "start",
				Name: "开始",
				Data: map[string]any{
					"inputs": []any{
						map[string]any{"name": "value"},
					},
				},
			},
			{
				ID:   "calc",
				Type: "tool",
				Name: "计算器",
				Data: map[string]any{
					"name": "calculator",
					"params": map[string]any{
						"expression": "{{#start.value#}} + 2",
					},
				},
			},
			{
				ID:   "end",
				Type: "end",
				Name: "结束",
				Data: map[string]any{
					"outputs": []any{
						map[string]any{"name": "result", "value": "{{#calc.result#}}"},
					},
				},
			},
		},
		Edges: []Edge{
			{ID: "edge-1", Source: "start", Target: "calc"},
			{ID: "edge-2", Source: "calc", Target: "end"},
		},
	}

	exec := GraphExecutor{}
	output, err := exec.Run(context.Background(), graph, map[string]any{"value": 3}, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if output == nil {
		t.Fatalf("expected output")
	}
	if output["result"] != float64(5) {
		t.Fatalf("expected result 5, got %v", output["result"])
	}
}
