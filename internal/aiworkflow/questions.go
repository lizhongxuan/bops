package aiworkflow

import (
	"context"
	"strings"
)

var missingQuestionMap = map[string]string{
	"targets":         "Which hosts or environments should this run on?",
	"hosts":           "Which hosts or groups should this run on?",
	"inventory":       "Do you want to use a specific inventory or environment?",
	"env":             "Do you need any environment variables or env packages?",
	"env_packages":    "Which env packages should be applied?",
	"validation_env":  "Which validation environment should be used?",
	"constraints":     "Any constraints such as OS, package manager, or network limits?",
	"resources":       "Any scripts, templates, or configs that should be used?",
	"actions":         "What actions or steps should be included?",
	"plan":            "Should the plan be manual-approve or auto?",
	"approval":        "Should this require manual approval?",
	"credentials":     "Any credentials or secrets needed (do not paste secrets here)?",
	"schedule":        "Is there a preferred schedule or window?",
	"requirements":    "Any additional requirements to include?",
	"output":          "What outputs should be verified?",
	"verification":    "How should the workflow be verified?",
	"rollback":        "Is a rollback or cleanup needed?",
	"scope":           "What is the scope or boundary of this change?",
	"dependencies":    "Are there dependencies or prerequisites to consider?",
	"service":         "Which service or package should be managed?",
	"config":          "Do you have config paths or template locations?",
	"command":         "What command should be executed?",
	"goal":            "What is the primary goal?",
	"description":     "Any description or context to include?",
	"steps":           "Are there specific steps or sequence requirements?",
	"risk":            "Any risk constraints or safeguards?",
	"timeout":         "Any timeout or retry requirements?",
	"language":        "What script language should be used?",
	"runtime":         "Any runtime or environment constraints?",
	"package_manager": "Which package manager should be used?",
	"os":              "Which OS or distribution should this target?",
	"version":         "Any version constraints?",
	"ports":           "Any ports or endpoints involved?",
	"files":           "Any files that need to be created or modified?",
	"paths":           "Any file paths to reference?",
	"users":           "Any users or permissions to consider?",
	"policy":          "Any policy or compliance requirements?",
	"monitoring":      "Should monitoring or alerts be added?",
	"notifications":   "Who should be notified?",
	"confirm":         "Is confirmation required for any step?",
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
		questions = append(questions, "Please provide: "+item+".")
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
