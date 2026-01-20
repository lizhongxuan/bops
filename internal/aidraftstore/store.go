package aidraftstore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bops/internal/ai"
	"gopkg.in/yaml.v3"
)

type Draft struct {
	YAML        string   `json:"yaml" yaml:"yaml"`
	Summary     string   `json:"summary" yaml:"summary"`
	Issues      []string `json:"issues" yaml:"issues"`
	RiskLevel   string   `json:"risk_level" yaml:"risk_level"`
	NeedsReview bool     `json:"needs_review" yaml:"needs_review"`
	History     []string `json:"history" yaml:"history"`
}

type Config struct {
	WorkflowName  string   `json:"workflow_name" yaml:"workflow_name"`
	Description   string   `json:"description" yaml:"description"`
	Targets       []string `json:"targets,omitempty" yaml:"targets,omitempty"`
	PlanMode      string   `json:"plan_mode,omitempty" yaml:"plan_mode,omitempty"`
	EnvPackages   []string `json:"env_packages,omitempty" yaml:"env_packages,omitempty"`
	ValidationEnv string   `json:"validation_env,omitempty" yaml:"validation_env,omitempty"`
	MaxRetries    int      `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
}

type Session struct {
	ID        string       `json:"id" yaml:"id"`
	Title     string       `json:"title" yaml:"title"`
	CreatedAt time.Time    `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" yaml:"updated_at"`
	Messages  []ai.Message `json:"messages" yaml:"messages"`
	Draft     Draft        `json:"draft" yaml:"draft"`
	Config    Config       `json:"config" yaml:"config"`
}

type Summary struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
	HasDraft     bool      `json:"has_draft"`
	RiskLevel    string    `json:"risk_level"`
}

type Store struct {
	Dir string
}

func New(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) List() ([]Summary, error) {
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
		id, ok := sessionIDFromFile(entry.Name())
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
			var session Session
			if err := yaml.Unmarshal(data, &session); err == nil {
				if strings.TrimSpace(session.Title) != "" {
					summary.Title = session.Title
				}
				summary.MessageCount = len(session.Messages)
				summary.HasDraft = strings.TrimSpace(session.Draft.YAML) != ""
				summary.RiskLevel = session.Draft.RiskLevel
				if !session.UpdatedAt.IsZero() {
					summary.UpdatedAt = session.UpdatedAt
				}
			}
		}

		items = append(items, summary)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	return items, nil
}

func (s *Store) Get(id string) (Session, []byte, error) {
	path, err := s.path(id)
	if err != nil {
		return Session{}, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, nil, err
	}

	var session Session
	if err := yaml.Unmarshal(data, &session); err != nil {
		return Session{}, nil, err
	}
	if session.ID == "" {
		session.ID = id
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = fileModTime(path)
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = fileModTime(path)
	}

	return session, data, nil
}

func (s *Store) Create(title string) (Session, error) {
	id := newSessionID()
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		trimmed = "AI Draft"
	}
	now := time.Now().UTC()
	session := Session{
		ID:        id,
		Title:     trimmed,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []ai.Message{},
		Draft:     Draft{},
		Config:    Config{},
	}

	if _, err := s.Save(session); err != nil {
		return Session{}, err
	}

	return session, nil
}

func (s *Store) Save(session Session) (Session, error) {
	if err := s.ensureDir(); err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(session.ID) == "" {
		return Session{}, fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(session.Title) == "" {
		session.Title = "AI Draft"
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	session.UpdatedAt = time.Now().UTC()

	path, err := s.path(session.ID)
	if err != nil {
		return Session{}, err
	}

	raw, err := yaml.Marshal(session)
	if err != nil {
		return Session{}, err
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return Session{}, err
	}

	return session, nil
}

func (s *Store) AppendMessage(id string, msg ai.Message) (Session, error) {
	session, _, err := s.Get(id)
	if err != nil {
		return Session{}, err
	}
	session.Messages = append(session.Messages, msg)
	return s.Save(session)
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

func sessionIDFromFile(filename string) (string, bool) {
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
		return "", fmt.Errorf("session id is empty")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid session id %q", id)
	}
	return trimmed, nil
}

func newSessionID() string {
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("draft-%s-%s", time.Now().UTC().Format("20060102-150405"), hex.EncodeToString(suffix))
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UTC()
	}
	return info.ModTime().UTC()
}
