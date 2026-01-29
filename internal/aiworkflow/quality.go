package aiworkflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"bops/internal/workflow"
	"gopkg.in/yaml.v3"
)

const (
	maxWithDepth = 3
)

type qualityResult struct {
	YAML    string
	Notices []string
}

func applyQualityGuardYAML(yamlText string) qualityResult {
	trimmed := strings.TrimSpace(yamlText)
	if trimmed == "" {
		return qualityResult{YAML: yamlText}
	}
	wf, err := workflow.Load([]byte(trimmed))
	if err != nil {
		return qualityResult{YAML: yamlText}
	}
	notices := make([]string, 0)

	if len(wf.Steps) > maxWorkflowStepCount {
		wf.Steps = wf.Steps[:maxWorkflowStepCount]
		notices = append(notices, fmt.Sprintf("步骤过多已截断为%d步", maxWorkflowStepCount))
	}

	changedByName := false
	for i := range wf.Steps {
		if strings.TrimSpace(wf.Steps[i].Name) == "" {
			if strings.TrimSpace(wf.Steps[i].Action) != "" {
				wf.Steps[i].Name = wf.Steps[i].Action
			} else {
				wf.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
			}
			changedByName = true
		}
	}
	if changedByName {
		notices = append(notices, "已补齐步骤名称")
	}

	changedByDepth := false
	for i := range wf.Steps {
		if wf.Steps[i].With == nil {
			continue
		}
		updated, changed := simplifyMapDepth(wf.Steps[i].With, 1, maxWithDepth)
		if changed {
			wf.Steps[i].With = updated
			changedByDepth = true
		}
	}
	if changedByDepth {
		notices = append(notices, "参数嵌套过深已简化")
	}

	simplified := dedupeIdenticalSteps(wf.Steps)
	if len(simplified) != len(wf.Steps) {
		wf.Steps = simplified
		notices = append(notices, "已合并重复步骤")
	}

	data, err := yaml.Marshal(wf)
	if err != nil {
		return qualityResult{YAML: yamlText, Notices: notices}
	}
	return qualityResult{YAML: strings.TrimSpace(string(data)), Notices: notices}
}

func simplifyMapDepth(input map[string]any, depth, maxDepth int) (map[string]any, bool) {
	changed := false
	out := make(map[string]any, len(input))
	for k, v := range input {
		updated, c := simplifyValueDepth(v, depth, maxDepth)
		if c {
			changed = true
		}
		out[k] = updated
	}
	return out, changed
}

func simplifyValueDepth(value any, depth, maxDepth int) (any, bool) {
	if depth >= maxDepth {
		switch value.(type) {
		case map[string]any, []any:
			if data, err := json.Marshal(value); err == nil {
				return string(data), true
			}
			return value, false
		}
		return value, false
	}

	switch v := value.(type) {
	case map[string]any:
		updated, changed := simplifyMapDepth(v, depth+1, maxDepth)
		return updated, changed
	case []any:
		changed := false
		out := make([]any, 0, len(v))
		for _, item := range v {
			updated, c := simplifyValueDepth(item, depth+1, maxDepth)
			if c {
				changed = true
			}
			out = append(out, updated)
		}
		return out, changed
	default:
		return value, false
	}
}

func dedupeIdenticalSteps(steps []workflow.Step) []workflow.Step {
	if len(steps) < 2 {
		return steps
	}
	result := make([]workflow.Step, 0, len(steps))
	var prevSignature string
	for _, step := range steps {
		sig := stepSignature(step)
		if sig == prevSignature {
			continue
		}
		result = append(result, step)
		prevSignature = sig
	}
	return result
}

func stepSignature(step workflow.Step) string {
	builder := strings.Builder{}
	builder.WriteString(strings.TrimSpace(step.Name))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(step.Action))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(step.When))
	builder.WriteString("|")
	builder.WriteString(step.Timeout)
	builder.WriteString("|")
	builder.WriteString(fmt.Sprintf("%d", step.Retries))
	builder.WriteString("|")
	builder.WriteString(strings.Join(step.Targets, ","))
	builder.WriteString("|")
	builder.WriteString(strings.Join(step.Notify, ","))
	builder.WriteString("|")
	builder.WriteString(sortedJSON(step.With))
	builder.WriteString("|")
	builder.WriteString(sortedJSON(step.Loop))
	return builder.String()
}

func sortedJSON(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := make(map[string]any, len(v))
		for _, k := range keys {
			ordered[k] = v[k]
		}
		if data, err := json.Marshal(ordered); err == nil {
			return string(data)
		}
	case []any:
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}
	default:
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}
	}
	if data, err := json.Marshal(value); err == nil {
		return string(data)
	}
	return ""
}
