package aiworkflow

import (
	"fmt"
	"strings"

	"bops/internal/workflow"
)

const (
	defaultPlanStepLimit     = 8
	complexStepMaxCandidates = 6
	complexScoreThreshold    = 2
)

type ComplexStep struct {
	Index  int
	Name   string
	Action string
	Reason string
}

func (s ComplexStep) Label() string {
	label := fmt.Sprintf("steps[%d]", s.Index)
	if strings.TrimSpace(s.Name) != "" {
		label += " " + s.Name
	}
	if strings.TrimSpace(s.Action) != "" {
		label += " (" + s.Action + ")"
	}
	if strings.TrimSpace(s.Reason) != "" {
		label += ": " + s.Reason
	}
	return label
}

func planStepLimit() int {
	if defaultPlanStepLimit > 0 && defaultPlanStepLimit < maxWorkflowStepCount {
		return defaultPlanStepLimit
	}
	return maxWorkflowStepCount
}

func detectComplexSteps(wf workflow.Workflow) []ComplexStep {
	complex := make([]ComplexStep, 0)
	for i, step := range wf.Steps {
		score, reason := complexityScore(step)
		if score >= complexScoreThreshold {
			complex = append(complex, ComplexStep{
				Index:  i,
				Name:   strings.TrimSpace(step.Name),
				Action: strings.TrimSpace(step.Action),
				Reason: reason,
			})
		}
		if len(complex) >= complexStepMaxCandidates {
			break
		}
	}
	return complex
}

func complexityScore(step workflow.Step) (int, string) {
	score := 0
	reasons := make([]string, 0, 3)
	action := strings.ToLower(strings.TrimSpace(step.Action))
	if action == "script.shell" || action == "script.python" || action == "template.render" {
		score++
		reasons = append(reasons, "action type")
	}
	if len(step.With) >= 3 {
		score++
		reasons = append(reasons, "many params")
	}
	if cmd, ok := step.With["cmd"].(string); ok {
		trimmed := strings.TrimSpace(cmd)
		if len(trimmed) > 80 || strings.Contains(trimmed, "&&") || strings.Contains(trimmed, ";") || strings.Contains(trimmed, "\n") {
			score++
			reasons = append(reasons, "complex cmd")
		}
	}
	if src, ok := step.With["src"].(string); ok {
		if strings.Contains(src, "/") && len(src) > 40 {
			score++
			reasons = append(reasons, "template path")
		}
	}
	if len(reasons) == 0 {
		return score, ""
	}
	return score, strings.Join(reasons, ", ")
}

func shouldOptimizePlan(wf workflow.Workflow) bool {
	if len(wf.Steps) == 0 {
		return false
	}
	if len(wf.Steps) >= maxWorkflowStepCount {
		return false
	}
	return len(detectComplexSteps(wf)) > 0
}
