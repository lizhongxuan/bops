package aiworkflow

import (
	"fmt"
	"strings"
	"sync"

	"bops/internal/workflow"
	"gopkg.in/yaml.v3"
)

type StateSnapshot struct {
	YAML    string
	Vars    map[string]any
	Plan    []PlanStep
	History []string
}

type StateStore struct {
	mu       sync.RWMutex
	yamlText string
	yamlNode *yaml.Node
	vars     map[string]any
	plan     []PlanStep
	history  []string
}

func NewStateStore(baseYAML string) *StateStore {
	store := &StateStore{}
	if baseYAML != "" {
		_ = store.setYAML(baseYAML)
	}
	return store
}

func (s *StateStore) UpdateYAMLFragment(fragment string, parentStepID string) error {
	if fragment == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.yamlText == "" {
		return s.setYAML(fragment)
	}
	parentStepID = strings.TrimSpace(parentStepID)
	next := ""
	if parentStepID == "" {
		next = mergeStepsIntoBase(s.yamlText, fragment)
	} else {
		merged, err := mergeFragmentIntoStep(s.yamlText, fragment)
		if err != nil {
			return err
		}
		next = merged
	}
	s.history = append(s.history, s.yamlText)
	return s.setYAML(next)
}

func (s *StateStore) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	varsCopy := map[string]any{}
	for key, value := range s.vars {
		varsCopy[key] = value
	}
	planCopy := append([]PlanStep{}, s.plan...)
	historyCopy := append([]string{}, s.history...)
	return StateSnapshot{YAML: s.yamlText, Vars: varsCopy, Plan: planCopy, History: historyCopy}
}

func (s *StateStore) SetPlan(plan []PlanStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plan = append([]PlanStep{}, plan...)
}

func (s *StateStore) SetVars(vars map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vars = map[string]any{}
	for key, value := range vars {
		s.vars[key] = value
	}
}

func (s *StateStore) setYAML(yamlText string) error {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(yamlText), &node); err != nil {
		return err
	}
	s.yamlText = yamlText
	s.yamlNode = &node
	return nil
}

func mergeFragmentIntoStep(baseYAML, fragment string) (string, error) {
	baseWorkflow, err := workflow.Load([]byte(strings.TrimSpace(baseYAML)))
	if err != nil {
		return "", err
	}
	steps, err := parseStepFragment(fragment)
	if err != nil {
		return "", err
	}
	for _, nextStep := range steps {
		if strings.TrimSpace(nextStep.Name) == "" {
			continue
		}
		updated := false
		for i, existing := range baseWorkflow.Steps {
			if strings.TrimSpace(existing.Name) == strings.TrimSpace(nextStep.Name) {
				baseWorkflow.Steps[i] = nextStep
				updated = true
				break
			}
		}
		if !updated {
			baseWorkflow.Steps = append(baseWorkflow.Steps, nextStep)
		}
	}
	baseWorkflow = normalizeWorkflow(baseWorkflow)
	return marshalWorkflowYAML(baseWorkflow)
}

func parseStepFragment(fragment string) ([]workflow.Step, error) {
	trimmed := strings.TrimSpace(fragment)
	if trimmed == "" {
		return nil, fmt.Errorf("fragment is empty")
	}
	lines := strings.Split(trimmed, "\n")
	indented := make([]string, 0, len(lines))
	for _, line := range lines {
		indented = append(indented, "  "+line)
	}
	payload := "steps:\n" + strings.Join(indented, "\n")
	var wrapper struct {
		Steps []workflow.Step `yaml:"steps"`
	}
	if err := yaml.Unmarshal([]byte(payload), &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Steps) == 0 {
		return nil, fmt.Errorf("no steps in fragment")
	}
	return wrapper.Steps, nil
}
