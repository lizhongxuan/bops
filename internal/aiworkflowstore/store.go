package aiworkflowstore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bops/internal/logging"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Revision struct {
	YAML      string    `json:"yaml" yaml:"yaml"`
	Summary   string    `json:"summary" yaml:"summary"`
	Diff      string    `json:"diff" yaml:"diff"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

type Draft struct {
	ID        string     `json:"id" yaml:"id"`
	Title     string     `json:"title" yaml:"title"`
	Prompt    string     `json:"prompt" yaml:"prompt"`
	YAML      string     `json:"yaml" yaml:"yaml"`
	Graph     string     `json:"graph" yaml:"graph"`
	Summary   string     `json:"summary" yaml:"summary"`
	Issues    []string   `json:"issues" yaml:"issues"`
	RiskLevel string     `json:"risk_level" yaml:"risk_level"`
	CreatedAt time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" yaml:"updated_at"`
	History   []Revision `json:"history" yaml:"history"`
}

type Summary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
	RiskLevel string    `json:"risk_level"`
	HasIssues bool      `json:"has_issues"`
}

type Store struct {
	Dir string
}

func New(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) List() ([]Summary, error) {
	logging.L().Debug("ai draft list", zap.String("dir", s.Dir))
	if err := s.ensureDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}

	items := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		id, ok := draftIDFromFile(entry.Name())
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		summary := Summary{
			ID:        id,
			UpdatedAt: info.ModTime().UTC(),
		}
		if data, err := os.ReadFile(filepath.Join(s.Dir, entry.Name())); err == nil {
			var draft Draft
			if err := yaml.Unmarshal(data, &draft); err == nil {
				summary.Title = draft.Title
				summary.RiskLevel = draft.RiskLevel
				summary.HasIssues = len(draft.Issues) > 0
				if !draft.UpdatedAt.IsZero() {
					summary.UpdatedAt = draft.UpdatedAt
				}
			}
		}

		items = append(items, summary)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	logging.L().Debug("ai draft list done", zap.Int("count", len(items)))
	return items, nil
}

func (s *Store) Get(id string) (Draft, []byte, error) {
	logging.L().Debug("ai draft get", zap.String("id", id))
	path, err := s.path(id)
	if err != nil {
		return Draft{}, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Draft{}, nil, err
	}

	var draft Draft
	if err := yaml.Unmarshal(data, &draft); err != nil {
		return Draft{}, nil, err
	}
	if draft.ID == "" {
		draft.ID = id
	}
	if draft.CreatedAt.IsZero() {
		draft.CreatedAt = fileModTime(path)
	}
	if draft.UpdatedAt.IsZero() {
		draft.UpdatedAt = fileModTime(path)
	}

	logging.L().Debug("ai draft get done", zap.String("id", draft.ID))
	return draft, data, nil
}

func (s *Store) Save(input Draft) (Draft, error) {
	logging.L().Debug("ai draft save",
		zap.String("id", input.ID),
		zap.Int("yaml_len", len(input.YAML)),
		zap.Int("issues", len(input.Issues)),
	)
	if err := s.ensureDir(); err != nil {
		return Draft{}, err
	}
	if strings.TrimSpace(input.ID) == "" {
		input.ID = newDraftID()
	}
	if strings.TrimSpace(input.Title) == "" {
		input.Title = "AI Draft"
	}

	var draft Draft
	if existing, _, err := s.Get(input.ID); err == nil {
		draft = existing
	}
	if draft.ID == "" {
		draft.ID = input.ID
		draft.CreatedAt = time.Now().UTC()
	}

	if strings.TrimSpace(input.Prompt) != "" {
		draft.Prompt = input.Prompt
	}
	if strings.TrimSpace(input.Title) != "" {
		draft.Title = input.Title
	}
	if strings.TrimSpace(input.YAML) != "" && input.YAML != draft.YAML {
		rev := Revision{
			YAML:      input.YAML,
			Summary:   input.Summary,
			Diff:      diffSummary(draft.YAML, input.YAML),
			CreatedAt: time.Now().UTC(),
		}
		draft.History = append(draft.History, rev)
	}

	if strings.TrimSpace(input.YAML) != "" {
		draft.YAML = input.YAML
	}
	if strings.TrimSpace(input.Graph) != "" {
		draft.Graph = input.Graph
	}
	if strings.TrimSpace(input.Summary) != "" {
		draft.Summary = input.Summary
	}
	if input.Issues != nil {
		draft.Issues = input.Issues
	}
	if input.RiskLevel != "" {
		draft.RiskLevel = input.RiskLevel
	}

	draft.UpdatedAt = time.Now().UTC()

	path, err := s.path(draft.ID)
	if err != nil {
		return Draft{}, err
	}
	data, err := yaml.Marshal(draft)
	if err != nil {
		return Draft{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Draft{}, err
	}

	logging.L().Debug("ai draft saved", zap.String("id", draft.ID))
	return draft, nil
}

func diffSummary(prev, next string) string {
	if strings.TrimSpace(prev) == "" {
		return "initial"
	}
	prevLines := strings.Split(prev, "\n")
	nextLines := strings.Split(next, "\n")
	added := 0
	removed := 0
	max := len(prevLines)
	if len(nextLines) > max {
		max = len(nextLines)
	}
	for i := 0; i < max; i++ {
		switch {
		case i >= len(prevLines):
			added++
		case i >= len(nextLines):
			removed++
		case prevLines[i] != nextLines[i]:
			added++
			removed++
		}
	}
	return fmt.Sprintf("+%d/-%d", added, removed)
}

func (s *Store) ensureDir() error {
	if strings.TrimSpace(s.Dir) == "" {
		return fmt.Errorf("store dir is empty")
	}
	return os.MkdirAll(s.Dir, 0o755)
}

func (s *Store) path(id string) (string, error) {
	safe, err := sanitizeID(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, safe+".yaml"), nil
}

func draftIDFromFile(filename string) (string, bool) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".yaml", ".yml":
		return base, true
	default:
		return "", false
	}
}

func sanitizeID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", fmt.Errorf("draft id is empty")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid draft id %q", id)
	}
	return trimmed, nil
}

func newDraftID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("draft-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UTC()
	}
	return info.ModTime().UTC()
}
