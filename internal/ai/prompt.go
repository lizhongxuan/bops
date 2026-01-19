package ai

import (
	"os"
	"strings"
)

const fallbackPrompt = "你是运维工作流编排助手。请只输出一份 YAML（不要解释），严格遵循给定 schema。"

func LoadPrompt(path string) string {
	if strings.TrimSpace(path) == "" {
		return fallbackPrompt
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fallbackPrompt
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return fallbackPrompt
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
