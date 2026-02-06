package aistore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"bops/internal/ai"
	"bops/runner/logging"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Session struct {
	ID        string          `json:"id" yaml:"id"`
	Title     string          `json:"title" yaml:"title"`
	CreatedAt time.Time       `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" yaml:"updated_at"`
	Messages  []ai.Message    `json:"messages" yaml:"messages"`
	Cards     []CardEntry     `json:"cards,omitempty" yaml:"cards,omitempty"`
	Timeline  []TimelineEntry `json:"timeline,omitempty" yaml:"timeline,omitempty"`
}

type CardEntry struct {
	ID        string         `json:"id" yaml:"id"`
	CardID    string         `json:"card_id,omitempty" yaml:"card_id,omitempty"`
	ReplyID   string         `json:"reply_id,omitempty" yaml:"reply_id,omitempty"`
	CardType  string         `json:"card_type,omitempty" yaml:"card_type,omitempty"`
	Payload   map[string]any `json:"payload" yaml:"payload"`
	CreatedAt time.Time      `json:"created_at" yaml:"created_at"`
}

type TimelineEntry struct {
	ID        string         `json:"id" yaml:"id"`
	Type      string         `json:"type" yaml:"type"` // message | card
	Role      string         `json:"role,omitempty" yaml:"role,omitempty"`
	Content   string         `json:"content,omitempty" yaml:"content,omitempty"`
	CardID    string         `json:"card_id,omitempty" yaml:"card_id,omitempty"`
	ReplyID   string         `json:"reply_id,omitempty" yaml:"reply_id,omitempty"`
	CardType  string         `json:"card_type,omitempty" yaml:"card_type,omitempty"`
	Payload   map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	CreatedAt time.Time      `json:"created_at" yaml:"created_at"`
}

type Summary struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

type Store struct {
	Dir string
	mu  sync.Mutex
}

func New(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) List() ([]Summary, error) {
	logging.L().Debug("ai session list", zap.String("dir", s.Dir))
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getUnlocked(id)
}

func (s *Store) getUnlocked(id string) (Session, []byte, error) {
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
		recovered, recErr := s.recoverCorruptedSession(path, id, data, err)
		if recErr == nil {
			return recovered, nil, nil
		}
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked(session)
}

func (s *Store) saveUnlocked(session Session) error {
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

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	logging.L().Debug("ai session saved", zap.String("id", session.ID))
	return nil
}

func (s *Store) AppendMessage(id string, msg ai.Message) (Session, error) {
	logging.L().Debug("ai session append message", zap.String("id", id), zap.String("role", msg.Role))
	s.mu.Lock()
	defer s.mu.Unlock()
	session, _, err := s.getUnlocked(id)
	if err != nil {
		return Session{}, err
	}
	session.Messages = append(session.Messages, msg)
	session.Timeline = append(session.Timeline, TimelineEntry{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Type:      "message",
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: time.Now().UTC(),
	})
	if err := s.saveUnlocked(session); err != nil {
		return Session{}, err
	}
	logging.L().Debug("ai session message appended", zap.String("id", id), zap.Int("messages", len(session.Messages)))
	return session, nil
}

func (s *Store) UpsertCard(id string, card CardEntry) (Session, error) {
	logging.L().Debug("ai session upsert card", zap.String("id", id), zap.String("card_id", card.CardID))
	s.mu.Lock()
	defer s.mu.Unlock()
	session, _, err := s.getUnlocked(id)
	if err != nil {
		return Session{}, err
	}
	if session.Cards == nil {
		session.Cards = []CardEntry{}
	}
	updated := false
	if card.CardID != "" {
		for i, existing := range session.Cards {
			if existing.CardID == card.CardID {
				session.Cards[i] = card
				updated = true
				break
			}
		}
	}
	if !updated {
		session.Cards = append(session.Cards, card)
	}
	if session.Timeline == nil {
		session.Timeline = []TimelineEntry{}
	}
	timelineUpdated := false
	if card.CardID != "" {
		for i, existing := range session.Timeline {
			if existing.Type == "card" && existing.CardID == card.CardID {
				session.Timeline[i].Payload = card.Payload
				session.Timeline[i].CardType = card.CardType
				session.Timeline[i].ReplyID = card.ReplyID
				timelineUpdated = true
				break
			}
		}
	}
	if !timelineUpdated {
		session.Timeline = append(session.Timeline, TimelineEntry{
			ID:        fmt.Sprintf("card-%d", time.Now().UnixNano()),
			Type:      "card",
			CardID:    card.CardID,
			ReplyID:   card.ReplyID,
			CardType:  card.CardType,
			Payload:   card.Payload,
			CreatedAt: time.Now().UTC(),
		})
	}
	if err := s.saveUnlocked(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Store) recoverCorruptedSession(path string, id string, data []byte, cause error) (Session, error) {
	logging.L().Error("ai session yaml corrupted, recovering",
		zap.String("id", id),
		zap.String("path", path),
		zap.Error(cause),
	)
	timestamp := time.Now().UTC().Format("20060102-150405")
	brokenPath := fmt.Sprintf("%s.broken-%s", path, timestamp)
	if err := os.WriteFile(brokenPath, data, 0o644); err == nil {
		_ = os.Remove(path)
	}
	now := time.Now().UTC()
	session := Session{
		ID:        id,
		Title:     "损坏会话(已恢复)",
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []ai.Message{},
		Cards:     []CardEntry{},
		Timeline:  []TimelineEntry{},
	}
	if err := s.saveUnlocked(session); err != nil {
		return Session{}, err
	}
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
