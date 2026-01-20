package aiworkflow

import (
	"encoding/json"
	"errors"
	"strings"

	"bops/internal/ai"
	"gopkg.in/yaml.v3"
)

func buildGeneratePrompt(prompt, contextText string) string {
	builder := strings.Builder{}
	if contextText != "" {
		builder.WriteString("Context:\n")
		builder.WriteString(contextText)
		builder.WriteString("\n\n")
	}
	builder.WriteString("User request:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\n")
	builder.WriteString("Return JSON only. Top-level keys: version, name, description, inventory, vars (optional), env_packages (optional), plan, steps, handlers (optional), tests (optional).\n")
	builder.WriteString("Do not include markdown or explanations.")
	return builder.String()
}

func buildFixPrompt(yamlText string, issues []string, lastError string) string {
	builder := strings.Builder{}
	builder.WriteString("Fix the YAML below and return JSON only with the same schema.\n\n")
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

func extractWorkflowYAML(reply string) (string, error) {
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return "", errors.New("empty ai response")
	}
	if jsonText := extractJSONBlock(trimmed); jsonText != "" {
		var payload any
		if err := json.Unmarshal([]byte(jsonText), &payload); err == nil {
			if out, err := yaml.Marshal(payload); err == nil {
				return strings.TrimSpace(string(out)), nil
			}
		}
	}
	fallback := strings.TrimSpace(ai.ExtractYAML(trimmed))
	if fallback == "" {
		return "", errors.New("unable to extract yaml")
	}
	return fallback, nil
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
