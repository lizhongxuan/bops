package aiworkflow

import (
	"strings"
	"sync"
	"time"
)

type DraftStore struct {
	mu     sync.RWMutex
	drafts map[string]*DraftState
}

func NewDraftStore() *DraftStore {
	return &DraftStore{drafts: map[string]*DraftState{}}
}

func (s *DraftStore) Get(id string) (*DraftState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	draft, ok := s.drafts[id]
	return draft, ok
}

func (s *DraftStore) GetOrCreate(id string, baseYAML string) *DraftState {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		trimmed = "draft"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if draft, ok := s.drafts[trimmed]; ok {
		return draft
	}
	draft := &DraftState{
		DraftID:   trimmed,
		Steps:     map[string]StepPatch{},
		Reviews:   map[string]ReviewResult{},
		Metrics:   map[string]int{},
		BaseYAML:  strings.TrimSpace(baseYAML),
		UpdatedAt: time.Now().UTC(),
	}
	s.drafts[trimmed] = draft
	return draft
}

func (s *DraftStore) UpdatePlan(id string, plan []PlanStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.drafts[id]
	if !ok {
		return
	}
	draft.Plan = append([]PlanStep{}, plan...)
	draft.UpdatedAt = time.Now().UTC()
}

func (s *DraftStore) UpdateStep(id string, patch StepPatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.drafts[id]
	if !ok {
		return
	}
	if draft.Steps == nil {
		draft.Steps = map[string]StepPatch{}
	}
	draft.Steps[patch.StepID] = patch
	draft.Metrics["steps_updated"] = draft.Metrics["steps_updated"] + 1
	draft.UpdatedAt = time.Now().UTC()
}

func (s *DraftStore) UpdateReview(id string, result ReviewResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.drafts[id]
	if !ok {
		return
	}
	if draft.Reviews == nil {
		draft.Reviews = map[string]ReviewResult{}
	}
	draft.Reviews[result.StepID] = result
	draft.Metrics["reviews"] = draft.Metrics["reviews"] + 1
	if result.Status == StepStatusFailed {
		draft.Metrics["review_failed"] = draft.Metrics["review_failed"] + 1
	}
	draft.UpdatedAt = time.Now().UTC()
}

func (s *DraftStore) AddMetric(id, key string, delta int) {
	if delta == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.drafts[id]
	if !ok {
		return
	}
	if draft.Metrics == nil {
		draft.Metrics = map[string]int{}
	}
	draft.Metrics[key] = draft.Metrics[key] + delta
	draft.UpdatedAt = time.Now().UTC()
}

func (s *DraftStore) Snapshot(id string) DraftState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	draft, ok := s.drafts[id]
	if !ok || draft == nil {
		return DraftState{}
	}
	out := DraftState{
		DraftID:   draft.DraftID,
		Plan:      append([]PlanStep{}, draft.Plan...),
		Steps:     map[string]StepPatch{},
		Reviews:   map[string]ReviewResult{},
		Metrics:   map[string]int{},
		BaseYAML:  draft.BaseYAML,
		UpdatedAt: draft.UpdatedAt,
	}
	for key, value := range draft.Steps {
		out.Steps[key] = value
	}
	for key, value := range draft.Reviews {
		out.Reviews[key] = value
	}
	for key, value := range draft.Metrics {
		out.Metrics[key] = value
	}
	return out
}
