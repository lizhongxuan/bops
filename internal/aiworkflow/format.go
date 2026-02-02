package aiworkflow

import (
	"errors"
	"fmt"
	"strings"

	"bops/internal/ai"
	"bops/internal/workflow"
)

func buildGeneratePrompt(prompt, contextText, baseYAML string) string {
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
	builder.WriteString("If information is missing, keep steps minimal and put questions in questions[].\n")
	builder.WriteString("Do not include markdown or explanations.")
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

func buildLoopPrompt(prompt, contextText, baseYAML string, toolNames, toolHistory []string, iteration int) string {
	builder := strings.Builder{}
	builder.WriteString("你是运维工作流自主循环 Agent。每轮只做一件事。\n")
	builder.WriteString(fmt.Sprintf("当前轮次: %d\n\n", iteration))
	if contextText != "" {
		builder.WriteString("上下文:\n")
		builder.WriteString(contextText)
		builder.WriteString("\n\n")
	}
	if trimmedBase := strings.TrimSpace(baseYAML); trimmedBase != "" {
		builder.WriteString("已有 workflow YAML:\n")
		builder.WriteString(trimmedBase)
		builder.WriteString("\n\n")
		builder.WriteString("请只修改 steps, 其他字段保持不变。\n\n")
	}
	builder.WriteString("用户需求:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\n")

	if len(toolNames) > 0 {
		builder.WriteString("可用工具:\n")
		for _, name := range toolNames {
			builder.WriteString("- ")
			builder.WriteString(name)
			builder.WriteString("\n")
		}
	} else {
		builder.WriteString("可用工具: 无\n")
	}
	if len(toolHistory) > 0 {
		builder.WriteString("工具历史:\n")
		for _, item := range tailStrings(toolHistory, 4) {
			builder.WriteString("- ")
			builder.WriteString(truncateRunes(item, 400))
			builder.WriteString("\n")
		}
	} else {
		builder.WriteString("工具历史: 无\n")
	}

	builder.WriteString("\n输出要求:\n")
	builder.WriteString("- 只返回 JSON, 不要解释或 Markdown。\n")
	builder.WriteString("- action 只能是 tool_call / final / need_more_info。\n")
	builder.WriteString("- 每轮只允许一个 action。\n")
	builder.WriteString("- tool_call 必须包含 tool 和 args。\n")
	builder.WriteString("- need_more_info 必须包含 questions 数组。\n")
	builder.WriteString("- final 必须包含 yaml 字段, 内容为完整 workflow YAML。\n")
	builder.WriteString("workflow YAML 约束:\n")
	builder.WriteString("- 顶层字段必须包含 version, name, description, inventory, plan, steps。\n")
	builder.WriteString("- steps 每项必须包含 name, action, with。\n")
	builder.WriteString("- steps 不要包含 targets。\n")
	builder.WriteString("- 只允许 action: ")
	builder.WriteString(allowedActionText())
	builder.WriteString(".\n")
	builder.WriteString("JSON 示例:\n")
	builder.WriteString("{\"action\":\"tool_call\",\"tool\":\"read_file\",\"args\":{\"path\":\"config.json\"}}\n")
	builder.WriteString("{\"action\":\"need_more_info\",\"questions\":[\"目标主机有哪些?\"]}\n")
	builder.WriteString("{\"action\":\"final\",\"yaml\":\"version: v0.1\\n...\"}\n")
	return builder.String()
}

func tailStrings(items []string, limit int) []string {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[len(items)-limit:]
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
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

func buildReviewPrompt(yamlText string, issues []string) string {
	builder := strings.Builder{}
	builder.WriteString("You are a workflow YAML reviewer. Return JSON only with keys: issues.\n")
	builder.WriteString("issues should be an array of strings. If no issues, return an empty array.\n\n")
	if len(issues) > 0 {
		builder.WriteString("Known issues:\n")
		for _, issue := range issues {
			builder.WriteString("- ")
			builder.WriteString(issue)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("YAML:\n")
	builder.WriteString(strings.TrimSpace(yamlText))
	builder.WriteString("\n\nReturn JSON only.")
	return builder.String()
}
