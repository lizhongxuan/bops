package aiworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	loopMemoryPRDFile        = "prd.json"
	loopMemoryProgressFile   = "progress.txt"
	loopMemoryCheckpointFile = "checkpoint.json"
)

type LoopMemoryStore interface {
	Load(ctx context.Context, sessionID string) (LoopMemorySnapshot, bool, error)
	Save(ctx context.Context, sessionID string, snapshot LoopMemorySnapshot) error
	AppendProgress(ctx context.Context, sessionID string, line string) error
	IsDurable() bool
	Name() string
}

type LoopMemorySnapshot struct {
	SessionID  string
	PRD        LoopPRD
	Progress   string
	Checkpoint LoopCheckpoint
}

type LoopPRD struct {
	BranchName  string         `json:"branch_name,omitempty"`
	UserStories []LoopPRDStory `json:"user_stories,omitempty"`
}

type LoopPRDStory struct {
	ID                 string   `json:"id,omitempty"`
	Title              string   `json:"title,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Priority           int      `json:"priority,omitempty"`
	Passes             bool     `json:"passes"`
	Notes              string   `json:"notes,omitempty"`
}

type LoopCheckpoint struct {
	Iteration        int      `json:"iteration,omitempty"`
	ToolCalls        int      `json:"tool_calls,omitempty"`
	ToolFailures     int      `json:"tool_failures,omitempty"`
	LastFingerprint  string   `json:"last_fingerprint,omitempty"`
	StableIterations int      `json:"stable_iterations,omitempty"`
	ToolHistory      []string `json:"tool_history,omitempty"`
	LastYAML         string   `json:"last_yaml,omitempty"`
	LastStopReasons  []string `json:"last_stop_reasons,omitempty"`
}

func defaultLoopMemorySnapshot(sessionID string) LoopMemorySnapshot {
	return LoopMemorySnapshot{
		SessionID: sessionID,
		PRD: LoopPRD{
			UserStories: []LoopPRDStory{},
		},
		Progress: "",
		Checkpoint: LoopCheckpoint{
			ToolHistory:     []string{},
			LastStopReasons: []string{},
		},
	}
}

type InMemoryLoopMemoryStore struct {
	mu   sync.Mutex
	data map[string]LoopMemorySnapshot
}

func NewInMemoryLoopMemoryStore() *InMemoryLoopMemoryStore {
	return &InMemoryLoopMemoryStore{
		data: make(map[string]LoopMemorySnapshot),
	}
}

func (s *InMemoryLoopMemoryStore) Name() string {
	return "memory"
}

func (s *InMemoryLoopMemoryStore) IsDurable() bool {
	return false
}

func (s *InMemoryLoopMemoryStore) Load(_ context.Context, sessionID string) (LoopMemorySnapshot, bool, error) {
	normalized := normalizeLoopSessionID(sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.data[normalized]
	if !ok {
		return defaultLoopMemorySnapshot(normalized), false, nil
	}
	return cloneLoopMemorySnapshot(snapshot), true, nil
}

func (s *InMemoryLoopMemoryStore) Save(_ context.Context, sessionID string, snapshot LoopMemorySnapshot) error {
	normalized := normalizeLoopSessionID(sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot.SessionID = normalized
	s.data[normalized] = cloneLoopMemorySnapshot(snapshot)
	return nil
}

func (s *InMemoryLoopMemoryStore) AppendProgress(ctx context.Context, sessionID string, line string) error {
	snapshot, _, err := s.Load(ctx, sessionID)
	if err != nil {
		return err
	}
	if snapshot.Progress != "" && !strings.HasSuffix(snapshot.Progress, "\n") {
		snapshot.Progress += "\n"
	}
	if strings.TrimSpace(line) != "" {
		snapshot.Progress += strings.TrimSpace(line) + "\n"
	}
	return s.Save(ctx, sessionID, snapshot)
}

type FileLoopMemoryStore struct {
	root string
}

func NewFileLoopMemoryStore(root string) *FileLoopMemoryStore {
	return &FileLoopMemoryStore{root: strings.TrimSpace(root)}
}

func (s *FileLoopMemoryStore) Name() string {
	return "file"
}

func (s *FileLoopMemoryStore) IsDurable() bool {
	return true
}

func (s *FileLoopMemoryStore) Load(_ context.Context, sessionID string) (LoopMemorySnapshot, bool, error) {
	if strings.TrimSpace(s.root) == "" {
		return LoopMemorySnapshot{}, false, fmt.Errorf("loop memory root is empty")
	}
	normalized := normalizeLoopSessionID(sessionID)
	dir := s.sessionDir(normalized)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return defaultLoopMemorySnapshot(normalized), false, nil
		}
		return LoopMemorySnapshot{}, false, fmt.Errorf("stat session dir: %w", err)
	}

	snapshot := defaultLoopMemorySnapshot(normalized)
	snapshot.SessionID = normalized

	prdPath := filepath.Join(dir, loopMemoryPRDFile)
	if data, err := os.ReadFile(prdPath); err == nil {
		if strings.TrimSpace(string(data)) != "" {
			if err := json.Unmarshal(data, &snapshot.PRD); err != nil {
				return LoopMemorySnapshot{}, true, fmt.Errorf("load %s: %w", loopMemoryPRDFile, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return LoopMemorySnapshot{}, true, fmt.Errorf("load %s: %w", loopMemoryPRDFile, err)
	}

	progressPath := filepath.Join(dir, loopMemoryProgressFile)
	if data, err := os.ReadFile(progressPath); err == nil {
		snapshot.Progress = string(data)
	} else if !os.IsNotExist(err) {
		return LoopMemorySnapshot{}, true, fmt.Errorf("load %s: %w", loopMemoryProgressFile, err)
	}

	checkpointPath := filepath.Join(dir, loopMemoryCheckpointFile)
	if data, err := os.ReadFile(checkpointPath); err == nil {
		if strings.TrimSpace(string(data)) != "" {
			if err := json.Unmarshal(data, &snapshot.Checkpoint); err != nil {
				return LoopMemorySnapshot{}, true, fmt.Errorf("load %s: %w", loopMemoryCheckpointFile, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return LoopMemorySnapshot{}, true, fmt.Errorf("load %s: %w", loopMemoryCheckpointFile, err)
	}

	return snapshot, true, nil
}

func (s *FileLoopMemoryStore) Save(_ context.Context, sessionID string, snapshot LoopMemorySnapshot) error {
	if strings.TrimSpace(s.root) == "" {
		return fmt.Errorf("loop memory root is empty")
	}
	normalized := normalizeLoopSessionID(sessionID)
	dir := s.sessionDir(normalized)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir session dir: %w", err)
	}
	snapshot.SessionID = normalized

	prdBytes, err := json.MarshalIndent(snapshot.PRD, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal prd: %w", err)
	}
	if err := writeAtomicFile(filepath.Join(dir, loopMemoryPRDFile), append(prdBytes, '\n')); err != nil {
		return err
	}
	if err := writeAtomicFile(filepath.Join(dir, loopMemoryProgressFile), []byte(snapshot.Progress)); err != nil {
		return err
	}
	checkpointBytes, err := json.MarshalIndent(snapshot.Checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	if err := writeAtomicFile(filepath.Join(dir, loopMemoryCheckpointFile), append(checkpointBytes, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *FileLoopMemoryStore) AppendProgress(ctx context.Context, sessionID string, line string) error {
	snapshot, _, err := s.Load(ctx, sessionID)
	if err != nil {
		return err
	}
	if snapshot.Progress != "" && !strings.HasSuffix(snapshot.Progress, "\n") {
		snapshot.Progress += "\n"
	}
	if strings.TrimSpace(line) != "" {
		snapshot.Progress += strings.TrimSpace(line) + "\n"
	}
	return s.Save(ctx, sessionID, snapshot)
}

func (s *FileLoopMemoryStore) sessionDir(sessionID string) string {
	return filepath.Join(s.root, normalizeLoopSessionID(sessionID))
}

func writeAtomicFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir for atomic write: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func normalizeLoopSessionID(sessionID string) string {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "default"
	}
	return out
}

func cloneLoopMemorySnapshot(in LoopMemorySnapshot) LoopMemorySnapshot {
	out := in
	out.PRD.UserStories = append([]LoopPRDStory{}, in.PRD.UserStories...)
	out.Checkpoint.ToolHistory = append([]string{}, in.Checkpoint.ToolHistory...)
	out.Checkpoint.LastStopReasons = append([]string{}, in.Checkpoint.LastStopReasons...)
	return out
}
