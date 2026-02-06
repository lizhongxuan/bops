package aiworkflow

import (
	"context"
	"strings"
)

var missingQuestionMap = map[string]string{
	"targets":         "请在 Inventory 页面补充目标主机/分组。",
	"hosts":           "请在 Inventory 页面补充目标主机/分组。",
	"inventory":       "请在 Inventory 页面补充主机/分组与变量配置。",
	"env":             "需要哪些环境变量或环境包？",
	"env_packages":    "需要安装哪些环境包？",
	"validation_env":  "使用哪个验证环境？",
	"constraints":     "是否有系统/包管理器/网络等限制？",
	"resources":       "是否有需要使用的脚本/模板/配置？",
	"actions":         "需要包含哪些动作或步骤？",
	"plan":            "计划模式使用手动审批还是自动？",
	"approval":        "是否需要人工审批？",
	"credentials":     "是否需要凭据或密钥（请不要粘贴明文）？",
	"schedule":        "是否有执行时间窗口？",
	"requirements":    "还有其他需求需要包含吗？",
	"output":          "需要验证哪些输出？",
	"verification":    "如何验证工作流结果？",
	"rollback":        "是否需要回滚或清理？",
	"scope":           "变更的范围/边界是什么？",
	"dependencies":    "是否有依赖或前置条件？",
	"service":         "需要管理哪个服务或包？",
	"config":          "配置路径或模板位置是什么？",
	"command":         "要执行的具体命令是什么？",
	"goal":            "主要目标是什么？",
	"description":     "需要补充的上下文描述？",
	"steps":           "对步骤顺序有要求吗？",
	"risk":            "是否有风险或安全约束？",
	"timeout":         "是否有超时或重试要求？",
	"language":        "脚本语言是什么？",
	"runtime":         "运行时/环境限制有哪些？",
	"package_manager": "使用哪个包管理器？",
	"os":              "目标操作系统/发行版？",
	"version":         "是否有版本要求？",
	"ports":           "涉及哪些端口或端点？",
	"files":           "需要创建或修改哪些文件？",
	"paths":           "涉及的文件路径有哪些？",
	"users":           "涉及哪些用户或权限？",
	"policy":          "是否有合规/策略要求？",
	"monitoring":      "是否需要监控或告警？",
	"notifications":   "需要通知哪些人？",
	"confirm":         "是否有必须确认的步骤？",
}

func (p *Pipeline) questionGate(_ context.Context, state *State) (*State, error) {
	if state.Mode != ModeGenerate {
		emitEvent(state, "question_gate", "skipped", "mode is not generate")
		return state, nil
	}
	emitEvent(state, "question_gate", "start", "")
	if state.Intent == nil || len(state.Intent.Missing) == 0 {
		emitEvent(state, "question_gate", "done", "")
		return state, nil
	}
	state.Questions = buildQuestionsFromMissing(state.Intent.Missing)
	state.RiskLevel = RiskLevelLow
	state.IsSuccess = true
	state.SkipExecute = true
	state.ExecutionSkipped = true
	emitEvent(state, "question_gate", "done", "awaiting missing inputs")
	return state, nil
}

func buildQuestionsFromMissing(missing []string) []string {
	cleaned := normalizeMissing(missing)
	questions := make([]string, 0, len(cleaned))
	for _, item := range cleaned {
		if question, ok := missingQuestionMap[item]; ok {
			questions = append(questions, question)
			continue
		}
		questions = append(questions, "请补充: "+item+"。")
	}
	return dedupeStrings(questions)
}

func normalizeMissing(missing []string) []string {
	cleaned := make([]string, 0, len(missing))
	for _, item := range missing {
		trimmed := strings.TrimSpace(strings.ToLower(item))
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return dedupeStrings(cleaned)
}

func mergeQuestions(existing, incoming []string) []string {
	if len(existing) == 0 {
		return normalizeQuestions(incoming)
	}
	if len(incoming) == 0 {
		return normalizeQuestions(existing)
	}
	combined := append(append([]string{}, existing...), incoming...)
	return normalizeQuestions(combined)
}

func normalizeQuestions(questions []string) []string {
	cleaned := make([]string, 0, len(questions))
	for _, item := range questions {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return dedupeStrings(cleaned)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
