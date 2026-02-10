package aiworkflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"bops/runner/workflow"
	"gopkg.in/yaml.v3"
)

const (
	defaultWorkflowVersion = "v0.1"
	defaultWorkflowName    = "draft-workflow"
	defaultPlanMode        = "manual-approve"
	defaultPlanStrategy    = "sequential"
	maxWorkflowStepCount   = 20
)

var allowedActionList = []string{
	"cmd.run",
	"template.render",
	"script.shell",
	"script.python",
	"env.set",
}

var allowedActionSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(allowedActionList))
	for _, action := range allowedActionList {
		set[action] = struct{}{}
	}
	return set
}()

type destructiveRule struct {
	reason string
	re     *regexp.Regexp
}

var destructiveRules = []destructiveRule{
	{reason: "destructive command detected: rm -rf /", re: regexp.MustCompile(`(?i)\brm\s+-rf\s+/`)},
	{reason: "destructive command detected: mkfs", re: regexp.MustCompile(`(?i)\bmkfs\b`)},
	{reason: "destructive command detected: shutdown or reboot", re: regexp.MustCompile(`(?i)\b(shutdown|reboot|poweroff|init\s+0)\b`)},
	{reason: "destructive command detected: wipefs or dd", re: regexp.MustCompile(`(?i)\b(wipefs|dd\s+if=.*of=/dev)\b`)},
}

type workflowEnvelope struct {
	Workflow  json.RawMessage `json:"workflow"`
	Questions []string        `json:"questions"`
}

func allowedActionText() string {
	return strings.Join(allowedActionList, ", ")
}

func isAllowedAction(action string) bool {
	_, ok := allowedActionSet[action]
	return ok
}

func parseWorkflowJSON(jsonText string) (workflow.Workflow, []string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return workflow.Workflow{}, nil, err
	}
	if len(raw) == 0 {
		return workflow.Workflow{}, nil, errors.New("empty workflow payload")
	}

	if _, ok := raw["workflow"]; ok {
		var envelope workflowEnvelope
		if err := json.Unmarshal([]byte(jsonText), &envelope); err != nil {
			return workflow.Workflow{}, nil, err
		}
		if len(envelope.Workflow) == 0 {
			return workflow.Workflow{}, normalizeQuestions(envelope.Questions), errors.New("workflow field is required")
		}
		var wf workflow.Workflow
		if err := json.Unmarshal(envelope.Workflow, &wf); err != nil {
			return workflow.Workflow{}, normalizeQuestions(envelope.Questions), err
		}
		return wf, normalizeQuestions(envelope.Questions), nil
	}

	if _, ok := raw["questions"]; ok {
		return workflow.Workflow{}, nil, errors.New("workflow field is required")
	}

	var wf workflow.Workflow
	if err := json.Unmarshal([]byte(jsonText), &wf); err != nil {
		return workflow.Workflow{}, nil, err
	}
	return wf, nil, nil
}

func normalizeWorkflow(wf workflow.Workflow) workflow.Workflow {
	if strings.TrimSpace(wf.Version) == "" {
		wf.Version = defaultWorkflowVersion
	}
	if strings.TrimSpace(wf.Name) == "" {
		wf.Name = defaultWorkflowName
	}
	if wf.Description == "" {
		wf.Description = ""
	}

	if len(wf.Inventory.Hosts) == 0 && len(wf.Inventory.Groups) == 0 {
		wf.Inventory.Hosts = map[string]workflow.Host{
			"local": {Address: "127.0.0.1"},
		}
	}

	if strings.TrimSpace(wf.Plan.Mode) == "" {
		wf.Plan.Mode = defaultPlanMode
	}
	if strings.TrimSpace(wf.Plan.Strategy) == "" {
		wf.Plan.Strategy = defaultPlanStrategy
	}

	for i := range wf.Steps {
		wf.Steps[i].Targets = nil
	}

	return wf
}

func marshalWorkflowYAML(wf workflow.Workflow) (string, error) {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func normalizeWorkflowYAML(yamlText string) string {
	trimmed := strings.TrimSpace(yamlText)
	if trimmed == "" {
		return ""
	}
	wf, err := workflow.Load([]byte(trimmed))
	if err != nil {
		return trimmed
	}
	wf = normalizeWorkflow(wf)
	out, err := marshalWorkflowYAML(wf)
	if err != nil {
		return trimmed
	}
	return out
}

func guardrailIssues(wf workflow.Workflow, yamlText string) []string {
	issues := []string{}
	if len(wf.Steps) > maxWorkflowStepCount {
		issues = append(issues, fmt.Sprintf("steps must be <= %d (got %d)", maxWorkflowStepCount, len(wf.Steps)))
	}
	for i, step := range wf.Steps {
		action := strings.TrimSpace(step.Action)
		if action == "" {
			continue
		}
		if !isAllowedAction(action) {
			issues = append(issues, fmt.Sprintf("steps[%d] action %q is not allowed", i, action))
		}
	}
	for _, rule := range destructiveRules {
		if rule.re.MatchString(yamlText) {
			issues = append(issues, rule.reason)
		}
	}
	return dedupeStrings(issues)
}

func forceManualApprove(yamlText string) (string, error) {
	trimmed := strings.TrimSpace(yamlText)
	if trimmed == "" {
		return "", errors.New("empty yaml")
	}
	wf, err := workflow.Load([]byte(trimmed))
	if err != nil {
		return "", err
	}
	wf = normalizeWorkflow(wf)
	wf.Plan.Mode = defaultPlanMode
	return marshalWorkflowYAML(wf)
}

func validateSubPlan(fragment string, step PlanStep) error {
	trimmed := strings.TrimSpace(fragment)
	if trimmed == "" {
		return errors.New("yaml fragment is empty")
	}
	for _, forbidden := range []string{"version:", "inventory:", "plan:"} {
		for _, line := range strings.Split(trimmed, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), forbidden) {
				return fmt.Errorf("sub plan must not include top-level %s", strings.TrimSuffix(forbidden, ":"))
			}
		}
	}
	if !strings.Contains(trimmed, "- name:") {
		return errors.New("sub plan must include a step name")
	}
	if strings.TrimSpace(step.StepName) != "" && !strings.Contains(trimmed, step.StepName) {
		return fmt.Errorf("sub plan must target step %q", step.StepName)
	}
	return nil
}
