package aiworkflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"bops/internal/workflow"
)

type NodeSpec struct {
	ID      string         `json:"id"`
	Index   int            `json:"index,omitempty"`
	Name    string         `json:"name"`
	Action  string         `json:"action"`
	With    map[string]any `json:"with"`
	Targets []string       `json:"targets,omitempty"`
}

type NeighborSpec struct {
	Name   string `json:"name"`
	Action string `json:"action"`
}

type NodeRegenResult struct {
	Step      workflow.Step
	Questions []string
}

type stepEnvelope struct {
	Step      workflow.Step `json:"step"`
	Questions []string      `json:"questions"`
}

func BuildNodeRegeneratePrompt(intent string, node NodeSpec, prev []NeighborSpec, next []NeighborSpec, yamlText string) string {
	builder := strings.Builder{}
	builder.WriteString("You are optimizing a single workflow step. Return JSON only.\n")
	builder.WriteString("Return JSON with keys: step, questions.\n")
	builder.WriteString("step must include: name, action, with. Do not include targets.\n")
	builder.WriteString("Allowed actions: ")
	builder.WriteString(allowedActionText())
	builder.WriteString(".\n")
	if strings.TrimSpace(intent) != "" {
		builder.WriteString("User intent:\n")
		builder.WriteString(strings.TrimSpace(intent))
		builder.WriteString("\n\n")
	}
	builder.WriteString("Current step:\n")
	builder.WriteString(fmt.Sprintf("name: %s\n", strings.TrimSpace(node.Name)))
	builder.WriteString(fmt.Sprintf("action: %s\n", strings.TrimSpace(node.Action)))
	if len(node.With) > 0 {
		builder.WriteString("with: ")
		if data, err := json.Marshal(node.With); err == nil {
			builder.WriteString(string(data))
		} else {
			builder.WriteString("{}")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
	if len(prev) > 0 {
		builder.WriteString("Previous steps:\n")
		for _, item := range prev {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(item.Name))
			if strings.TrimSpace(item.Action) != "" {
				builder.WriteString(" (" + strings.TrimSpace(item.Action) + ")")
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	if len(next) > 0 {
		builder.WriteString("Next steps:\n")
		for _, item := range next {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(item.Name))
			if strings.TrimSpace(item.Action) != "" {
				builder.WriteString(" (" + strings.TrimSpace(item.Action) + ")")
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("Workflow YAML (context):\n")
	builder.WriteString(strings.TrimSpace(yamlText))
	builder.WriteString("\n\nReturn JSON only. Do not include markdown or explanations.")
	return builder.String()
}

func ParseNodeRegenResponse(reply string) (NodeRegenResult, error) {
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		return NodeRegenResult{}, errors.New("node regen response is not json")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return NodeRegenResult{}, err
	}
	if len(raw) == 0 {
		return NodeRegenResult{}, errors.New("empty node regen payload")
	}
	if _, ok := raw["step"]; ok {
		var envelope stepEnvelope
		if err := json.Unmarshal([]byte(jsonText), &envelope); err != nil {
			return NodeRegenResult{}, err
		}
		envelope.Questions = normalizeQuestions(envelope.Questions)
		return NodeRegenResult{Step: envelope.Step, Questions: envelope.Questions}, nil
	}
	if _, ok := raw["questions"]; ok {
		return NodeRegenResult{}, errors.New("step field is required")
	}
	var step workflow.Step
	if err := json.Unmarshal([]byte(jsonText), &step); err != nil {
		return NodeRegenResult{}, err
	}
	return NodeRegenResult{Step: step}, nil
}

func NormalizeNodeRegenStep(step workflow.Step) (workflow.Step, []string) {
	issues := []string{}
	step.Name = strings.TrimSpace(step.Name)
	step.Action = strings.TrimSpace(step.Action)
	if step.Name == "" {
		issues = append(issues, "step name is required")
	}
	if step.Action == "" {
		issues = append(issues, "step action is required")
	} else if !isAllowedAction(step.Action) {
		issues = append(issues, fmt.Sprintf("step action %q is not allowed", step.Action))
	}
	return step, issues
}
