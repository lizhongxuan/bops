package aiworkflow

import (
	"fmt"
	"sort"
	"strings"

	"bops/runner/workflow"
)

type SimulationStep struct {
	Index   int            `json:"index"`
	Name    string         `json:"name"`
	Action  string         `json:"action"`
	Targets []string       `json:"targets,omitempty"`
	Args    map[string]any `json:"args,omitempty"`
	When    string         `json:"when,omitempty"`
}

type SimulationResult struct {
	Summary      string           `json:"summary"`
	PlanMode     string           `json:"plan_mode,omitempty"`
	PlanStrategy string           `json:"plan_strategy,omitempty"`
	Hosts        []string         `json:"hosts,omitempty"`
	Steps        []SimulationStep `json:"steps"`
	Vars         map[string]any   `json:"vars,omitempty"`
}

func RunSimulation(yamlText string, vars map[string]any) (*SimulationResult, error) {
	trimmed := strings.TrimSpace(yamlText)
	if trimmed == "" {
		return nil, fmt.Errorf("yaml is empty")
	}
	wf, err := workflow.Load([]byte(trimmed))
	if err != nil {
		return nil, err
	}
	wf = normalizeWorkflow(wf)

	steps := make([]SimulationStep, 0, len(wf.Steps))
	for i, step := range wf.Steps {
		steps = append(steps, SimulationStep{
			Index:   i + 1,
			Name:    strings.TrimSpace(step.Name),
			Action:  strings.TrimSpace(step.Action),
			Targets: append([]string{}, step.Targets...),
			Args:    step.Args,
			When:    strings.TrimSpace(step.When),
		})
	}
	hosts := collectHosts(wf.Inventory)
	result := &SimulationResult{
		PlanMode:     strings.TrimSpace(wf.Plan.Mode),
		PlanStrategy: strings.TrimSpace(wf.Plan.Strategy),
		Hosts:        hosts,
		Steps:        steps,
		Vars:         vars,
	}
	result.Summary = fmt.Sprintf("steps=%d hosts=%d plan=%s/%s", len(steps), len(hosts), result.PlanMode, result.PlanStrategy)
	return result, nil
}

func collectHosts(inv workflow.Inventory) []string {
	seen := make(map[string]struct{})
	for name := range inv.Hosts {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}
	for _, group := range inv.Groups {
		for _, host := range group.Hosts {
			trimmed := strings.TrimSpace(host)
			if trimmed != "" {
				seen[trimmed] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
