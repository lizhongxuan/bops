package aiworkflow

import (
	"errors"
	"fmt"
	"strings"

	"bops/internal/ai"
	"bops/internal/workflow"
)

func buildPlanPrompt(prompt, contextText, baseYAML string) string {
	builder := strings.Builder{}
	if contextText != "" {
		builder.WriteString("Context:\n")
		builder.WriteString(contextText)
		builder.WriteString("\n\n")
	}
	trimmedBase := strings.TrimSpace(baseYAML)
	if trimmedBase != "" {
		builder.WriteString("Existing workflow YAML:\n")
		builder.WriteString(trimmedBase)
		builder.WriteString("\n\n")
		builder.WriteString("Update the existing YAML based on the user request.\n")
		builder.WriteString("Only modify the steps section. Do not change any other fields.\n")
		builder.WriteString("If steps already exist, edit or append to them instead of creating a brand new workflow.\n\n")
	}
	builder.WriteString("User request:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\n")
	builder.WriteString("Return JSON only with top-level keys: workflow, questions.\n")
	builder.WriteString("workflow must include: version, name, description, inventory, plan, steps.\n")
	builder.WriteString("Steps must include name/action/with and must not include targets.\n")
	builder.WriteString("Allowed actions: ")
	builder.WriteString(allowedActionText())
	builder.WriteString(".\n")
	builder.WriteString("First, produce a high-level plan with minimal steps (keep it short, target <= ")
	builder.WriteString(fmt.Sprintf("%d", planStepLimit()))
	builder.WriteString(").\n")
	builder.WriteString("If information is missing, keep steps minimal and put questions in questions[].\n")
	builder.WriteString("Do not include markdown or explanations.")
	return builder.String()
}

func buildOptimizePrompt(planYAML string, complexSteps []ComplexStep) string {
	builder := strings.Builder{}
	builder.WriteString("You are optimizing a workflow plan. Return JSON only with top-level keys: workflow, questions.\n")
	builder.WriteString("workflow must include: version, name, description, inventory, plan, steps.\n")
	builder.WriteString("Steps must include name/action/with and must not include targets.\n")
	builder.WriteString("Allowed actions: ")
	builder.WriteString(allowedActionText())
	builder.WriteString(".\n")
	builder.WriteString("Only refine the complex steps listed below. Keep other steps unchanged.\n")
	builder.WriteString("Do not increase total steps beyond ")
	builder.WriteString(fmt.Sprintf("%d", maxWorkflowStepCount))
	builder.WriteString(".\n")
	builder.WriteString("Complex steps:\n")
	for _, step := range complexSteps {
		builder.WriteString("- ")
		builder.WriteString(step.Label())
		builder.WriteString("\n")
	}
	builder.WriteString("\nPlan YAML:\n")
	builder.WriteString(strings.TrimSpace(planYAML))
	builder.WriteString("\n\nReturn JSON only. Do not include markdown or explanations.")
	return builder.String()
}

func buildFixPrompt(yamlText string, issues []string, lastError string) string {
	builder := strings.Builder{}
	builder.WriteString("Fix the YAML below and return JSON only with top-level keys: workflow, questions.\n")
	builder.WriteString("workflow must include: version, name, description, inventory, plan, steps.\n")
	builder.WriteString("Steps must include name/action/with and must not include targets.\n")
	builder.WriteString("Only modify steps. Do not change any other fields.\n")
	builder.WriteString("Allowed actions: ")
	builder.WriteString(allowedActionText())
	builder.WriteString(".\n\n")
	builder.WriteString("YAML:\n")
	builder.WriteString(yamlText)
	builder.WriteString("\n\n")
	if len(issues) > 0 {
		builder.WriteString("Issues:\n")
		for _, issue := range issues {
			builder.WriteString("- ")
			builder.WriteString(issue)
			builder.WriteString("\n")
		}
	}
	if strings.TrimSpace(lastError) != "" {
		builder.WriteString("\nExecution error:\n")
		builder.WriteString(lastError)
		builder.WriteString("\n")
	}
	builder.WriteString("\nReturn JSON only. Do not include markdown.")
	return builder.String()
}

func extractWorkflowYAML(reply string) (string, []string, error) {
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return "", nil, errors.New("empty ai response")
	}
	if jsonText := extractJSONBlock(trimmed); jsonText != "" {
		workflowPayload, questions, err := parseWorkflowJSON(jsonText)
		if err == nil {
			workflowPayload = normalizeWorkflow(workflowPayload)
			if out, err := marshalWorkflowYAML(workflowPayload); err == nil {
				return out, questions, nil
			}
		}
	}
	fallback := strings.TrimSpace(ai.ExtractYAML(trimmed))
	if fallback == "" {
		return "", nil, errors.New("unable to extract yaml")
	}
	return normalizeWorkflowYAML(fallback), nil, nil
}

func mergeStepsIntoBase(baseYAML, updatedYAML string) string {
	trimmedBase := strings.TrimSpace(baseYAML)
	trimmedUpdated := strings.TrimSpace(updatedYAML)
	if trimmedBase == "" {
		return trimmedUpdated
	}
	if trimmedUpdated == "" {
		return trimmedBase
	}
	baseWorkflow, err := workflow.Load([]byte(trimmedBase))
	if err != nil {
		return trimmedUpdated
	}
	updatedWorkflow, err := workflow.Load([]byte(trimmedUpdated))
	if err != nil {
		return trimmedUpdated
	}
	updatedWorkflow = normalizeWorkflow(updatedWorkflow)
	baseWorkflow.Steps = updatedWorkflow.Steps
	out, err := marshalWorkflowYAML(baseWorkflow)
	if err != nil {
		return trimmedUpdated
	}
	return out
}

func extractJSONBlock(text string) string {
	if block := extractCodeBlock(text); block != "" {
		text = block
	}
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return trimmed
	}
	return ""
}

func extractCodeBlock(text string) string {
	lines := strings.Split(text, "\n")
	inBlock := false
	block := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inBlock {
				inBlock = true
				block = block[:0]
				continue
			}
			return strings.Join(block, "\n")
		}
		if inBlock {
			block = append(block, line)
		}
	}
	return ""
}
