package aiworkflow

import (
	"context"
	"sync"
)

type CheckPointStore struct {
	mu    sync.Mutex
	store map[string][]byte
}

func NewCheckPointStore() *CheckPointStore {
	return &CheckPointStore{
		store: make(map[string][]byte),
	}
}

func (s *CheckPointStore) Get(_ context.Context, checkPointID string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.store[checkPointID]
	if !ok {
		return nil, false, nil
	}
	clone := make([]byte, len(data))
	copy(clone, data)
	return clone, true, nil
}

func (s *CheckPointStore) Set(_ context.Context, checkPointID string, checkPoint []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := make([]byte, len(checkPoint))
	copy(clone, checkPoint)
	s.store[checkPointID] = clone
	return nil
}
