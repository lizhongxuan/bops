package aiworkflow

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type AgentManager struct {
	mu     sync.RWMutex
	agents map[string]AgentSpec
}

func NewAgentManager() *AgentManager {
	m := &AgentManager{agents: make(map[string]AgentSpec)}
	_ = m.Register(AgentSpec{Name: "main", Role: "primary"})
	return m
}

func (m *AgentManager) Register(spec AgentSpec) error {
	if m == nil {
		return fmt.Errorf("agent manager is nil")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return fmt.Errorf("agent name is required")
	}
	spec.Name = name
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[name] = spec
	return nil
}

func (m *AgentManager) Get(name string) (AgentSpec, bool) {
	if m == nil {
		return AgentSpec{}, false
	}
	key := strings.TrimSpace(name)
	if key == "" {
		return AgentSpec{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	spec, ok := m.agents[key]
	return spec, ok
}

func (m *AgentManager) List() []AgentSpec {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AgentSpec, 0, len(m.agents))
	for _, spec := range m.agents {
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
