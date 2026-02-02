package ai

import (
	"os"
	"strings"
)

const fallbackPrompt = "你是运维工作流编排助手。请只输出一份 YAML（不要解释），严格遵循给定 schema。"
const fallbackLoopPrompt = "你是运维工作流自主循环 Agent。每轮只做一件事，输出必须是 JSON，并且 action 只能是 tool_call / final / need_more_info。"

func LoadPrompt(path string) string {
	return loadPromptWithFallback(path, fallbackPrompt)
}

func LoadLoopPrompt(path string) string {
	return loadPromptWithFallback(path, fallbackLoopPrompt)
}

func loadPromptWithFallback(path, fallback string) string {
	if strings.TrimSpace(path) == "" {
		return fallback
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return fallback
	}
	return text
}

func ExtractYAML(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	var (
		inBlock bool
		block   []string
	)
	for _, line := range lines {
		lineTrim := strings.TrimSpace(line)
		if strings.HasPrefix(lineTrim, "```") {
			if !inBlock {
				inBlock = true
				block = block[:0]
				continue
			}
			if inBlock {
				return strings.TrimSpace(strings.Join(block, "\n"))
			}
		}
		if inBlock {
			block = append(block, line)
		}
	}
	return trimmed
}

func SummarizeToolOutput(output string, maxRunes int) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	if maxRunes <= 0 {
		maxRunes = 400
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	return string(runes[:maxRunes]) + "..."
}
