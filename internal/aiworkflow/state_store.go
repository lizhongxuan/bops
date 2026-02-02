package aiworkflow

import (
	"sync"

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

func (s *StateStore) UpdateYAMLFragment(fragment string) error {
	if fragment == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.yamlText == "" {
		return s.setYAML(fragment)
	}
	next := mergeStepsIntoBase(s.yamlText, fragment)
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
