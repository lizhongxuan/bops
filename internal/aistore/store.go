package aistore

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
	"bops/internal/logging"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Session struct {
	ID        string       `json:"id" yaml:"id"`
	Title     string       `json:"title" yaml:"title"`
	CreatedAt time.Time    `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" yaml:"updated_at"`
	Messages  []ai.Message `json:"messages" yaml:"messages"`
}

type Summary struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

type Store struct {
	Dir string
}

func New(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) List() ([]Summary, error) {
	logging.L().Debug("ai session list", zap.String("dir", s.Dir))
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

	logging.L().Debug("ai session list done", zap.Int("count", len(items)))
	return items, nil
}

func (s *Store) Get(id string) (Session, []byte, error) {
	logging.L().Debug("ai session get", zap.String("id", id))
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

	logging.L().Debug("ai session get done", zap.String("id", session.ID))
	return session, data, nil
}

func (s *Store) Create(title string) (Session, error) {
	logging.L().Debug("ai session create", zap.String("title", title))
	id := newSessionID()
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		trimmed = "新会话"
	}
	now := time.Now().UTC()
	session := Session{
		ID:        id,
		Title:     trimmed,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []ai.Message{},
	}

	if err := s.Save(session); err != nil {
		return Session{}, err
	}

	logging.L().Debug("ai session created", zap.String("id", session.ID))
	return session, nil
}

func (s *Store) Save(session Session) error {
	logging.L().Debug("ai session save", zap.String("id", session.ID), zap.Int("messages", len(session.Messages)))
	if err := s.ensureDir(); err != nil {
		return err
	}
	if strings.TrimSpace(session.ID) == "" {
		return fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(session.Title) == "" {
		session.Title = "新会话"
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	session.UpdatedAt = time.Now().UTC()

	path, err := s.path(session.ID)
	if err != nil {
		return err
	}

	raw, err := yaml.Marshal(session)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}
	logging.L().Debug("ai session saved", zap.String("id", session.ID))
	return nil
}

func (s *Store) AppendMessage(id string, msg ai.Message) (Session, error) {
	logging.L().Debug("ai session append message", zap.String("id", id), zap.String("role", msg.Role))
	session, _, err := s.Get(id)
	if err != nil {
		return Session{}, err
	}
	session.Messages = append(session.Messages, msg)
	if err := s.Save(session); err != nil {
		return Session{}, err
	}
	logging.L().Debug("ai session message appended", zap.String("id", id), zap.Int("messages", len(session.Messages)))
	return session, nil
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
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("session-%s-%s", time.Now().UTC().Format("20060102-150405"), hex.EncodeToString(suffix))
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UTC()
	}
	return info.ModTime().UTC()
}
