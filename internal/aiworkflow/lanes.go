package aiworkflow

import (
	"context"
	"sync"
)

const defaultGlobalLaneConcurrency = 4

type SessionLane struct {
	mu    sync.Mutex
	lanes map[string]chan struct{}
}

func NewSessionLane() *SessionLane {
	return &SessionLane{lanes: make(map[string]chan struct{})}
}

func (s *SessionLane) Do(ctx context.Context, key string, fn func() error) error {
	if s == nil || key == "" || fn == nil {
		if fn == nil {
			return nil
		}
		return fn()
	}
	ch := s.lane(key)
	select {
	case ch <- struct{}{}:
		defer func() { <-ch }()
		return fn()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *SessionLane) lane(key string) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lanes == nil {
		s.lanes = make(map[string]chan struct{})
	}
	ch, ok := s.lanes[key]
	if !ok {
		ch = make(chan struct{}, 1)
		s.lanes[key] = ch
	}
	return ch
}

type GlobalLane struct {
	sem chan struct{}
}

func NewGlobalLane(limit int) *GlobalLane {
	if limit <= 0 {
		limit = 1
	}
	return &GlobalLane{sem: make(chan struct{}, limit)}
}

func (g *GlobalLane) Do(ctx context.Context, fn func() error) error {
	if g == nil || g.sem == nil || fn == nil {
		if fn == nil {
			return nil
		}
		return fn()
	}
	select {
	case g.sem <- struct{}{}:
		defer func() { <-g.sem }()
		return fn()
	case <-ctx.Done():
		return ctx.Err()
	}
}
