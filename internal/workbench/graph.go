package workbench

import (
	"encoding/json"
	"fmt"
	"strings"

	"bops/internal/workflow"
	"gopkg.in/yaml.v3"
)

type Graph struct {
	Version string      `json:"version"`
	Layout  GraphLayout `json:"layout,omitempty"`
	Nodes   []Node      `json:"nodes"`
	Edges   []Edge      `json:"edges"`
}

type GraphLayout struct {
	Direction string `json:"direction,omitempty"`
}

type Node struct {
	ID   string         `json:"id"`
	Type string         `json:"type"`
	Name string         `json:"name"`
	Data map[string]any `json:"data,omitempty"`
	Meta map[string]any `json:"meta,omitempty"`
	UI   NodeUI         `json:"ui,omitempty"`
}

type NodeUI struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type Edge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

func GraphFromWorkflow(wf workflow.Workflow) Graph {
	nodes := make([]Node, 0, len(wf.Steps))
	edges := make([]Edge, 0)
	for i, step := range wf.Steps {
		id := fmt.Sprintf("step-%d", i+1)
		name := strings.TrimSpace(step.Name)
		if name == "" {
			name = id
		}
		node := Node{
			ID:      id,
			Type:    "action",
			Name:    name,
			Data: map[string]any{
				"action":  strings.TrimSpace(step.Action),
				"with":    step.With,
				"targets": append([]string{}, step.Targets...),
			},
			UI: NodeUI{
				X: float64(240 * i),
				Y: 80,
			},
		}
		nodes = append(nodes, node)
		if i > 0 {
			edges = append(edges, Edge{
				ID:     fmt.Sprintf("edge-%d", i),
				Source: nodes[i-1].ID,
				Target: node.ID,
			})
		}
	}
	return Graph{
		Version: "v1",
		Layout: GraphLayout{
			Direction: "LR",
		},
		Nodes: nodes,
		Edges: edges,
	}
}

func GraphFromYAML(yamlText string) (Graph, error) {
	trimmed := strings.TrimSpace(yamlText)
	if trimmed == "" {
		return Graph{}, fmt.Errorf("empty yaml")
	}
	wf, err := workflow.Load([]byte(trimmed))
	if err != nil {
		return Graph{}, err
	}
	return GraphFromWorkflow(wf), nil
}

func WorkflowFromGraph(graph Graph, base workflow.Workflow) workflow.Workflow {
	order := topoOrder(graph)
	nodeMap := make(map[string]Node, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeMap[node.ID] = node
	}
	steps := make([]workflow.Step, 0, len(order))
	for _, id := range order {
		node, ok := nodeMap[id]
		if !ok {
			continue
		}
		if strings.TrimSpace(node.Type) != "action" {
			continue
		}
		action := ""
		var with map[string]any
		var targets []string
		if node.Data != nil {
			if raw, ok := node.Data["action"]; ok {
				action = strings.TrimSpace(fmt.Sprint(raw))
			}
			if raw, ok := node.Data["with"]; ok {
				if mapped, ok := raw.(map[string]any); ok {
					with = mapped
				}
			}
			if raw, ok := node.Data["targets"]; ok {
				switch v := raw.(type) {
				case []string:
					targets = append([]string{}, v...)
				case []any:
					for _, item := range v {
						targets = append(targets, fmt.Sprint(item))
					}
				}
			}
		}
		step := workflow.Step{
			Name:    node.Name,
			Action:  action,
			With:    with,
			Targets: targets,
		}
		steps = append(steps, step)
	}
	base.Steps = steps
	return base
}

func ApplyGraphToYAML(graph Graph, yamlText string) (string, error) {
	trimmed := strings.TrimSpace(yamlText)
	if trimmed == "" {
		return "", fmt.Errorf("empty yaml")
	}
	wf, err := workflow.Load([]byte(trimmed))
	if err != nil {
		return "", err
	}
	wf = WorkflowFromGraph(graph, wf)
	data, err := yaml.Marshal(wf)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func ParseGraphJSON(data []byte) (Graph, error) {
	var graph Graph
	if err := json.Unmarshal(data, &graph); err != nil {
		return Graph{}, err
	}
	if strings.TrimSpace(graph.Version) == "" {
		graph.Version = "v1"
	}
	return graph, nil
}

func topoOrder(graph Graph) []string {
	index := make(map[string]int, len(graph.Nodes))
	indeg := make(map[string]int, len(graph.Nodes))
	adj := make(map[string][]string, len(graph.Nodes))
	for i, node := range graph.Nodes {
		index[node.ID] = i
		indeg[node.ID] = 0
		adj[node.ID] = nil
	}
	for _, edge := range graph.Edges {
		if _, ok := indeg[edge.Source]; !ok {
			continue
		}
		if _, ok := indeg[edge.Target]; !ok {
			continue
		}
		indeg[edge.Target]++
		adj[edge.Source] = append(adj[edge.Source], edge.Target)
	}

	queue := make([]string, 0)
	for _, node := range graph.Nodes {
		if indeg[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}

	order := make([]string, 0, len(graph.Nodes))
	for len(queue) > 0 {
		minIdx := 0
		for i := 1; i < len(queue); i++ {
			if index[queue[i]] < index[queue[minIdx]] {
				minIdx = i
			}
		}
		id := queue[minIdx]
		queue = append(queue[:minIdx], queue[minIdx+1:]...)
		order = append(order, id)
		for _, next := range adj[id] {
			indeg[next]--
			if indeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(graph.Nodes) {
		fallback := make([]string, 0, len(graph.Nodes))
		for _, node := range graph.Nodes {
			fallback = append(fallback, node.ID)
		}
		return fallback
	}

	return order
}
