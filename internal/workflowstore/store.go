package workflowstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bops/internal/workflow"
)

type Summary struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
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
		name, ok := workflowNameFromFile(entry.Name())
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		summary := Summary{
			Name:      name,
			UpdatedAt: info.ModTime(),
		}

		if data, err := os.ReadFile(filepath.Join(s.Dir, entry.Name())); err == nil {
			if wf, err := workflow.Load(data); err == nil {
				if strings.TrimSpace(wf.Description) != "" {
					summary.Description = wf.Description
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

func (s *Store) Get(name string) (workflow.Workflow, []byte, error) {
	path, err := s.path(name)
	if err != nil {
		return workflow.Workflow{}, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return workflow.Workflow{}, nil, err
	}

	wf, err := workflow.Load(data)
	if err != nil {
		return workflow.Workflow{}, nil, err
	}

	return wf, data, nil
}

func (s *Store) Put(name string, raw []byte) (workflow.Workflow, error) {
	if err := s.ensureDir(); err != nil {
		return workflow.Workflow{}, err
	}

	wf, err := workflow.Load(raw)
	if err != nil {
		return workflow.Workflow{}, err
	}

	if strings.TrimSpace(name) == "" {
		name = wf.Name
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return workflow.Workflow{}, fmt.Errorf("workflow name is required")
	}
	if wf.Name != "" && wf.Name != name {
		return workflow.Workflow{}, fmt.Errorf("workflow name mismatch: %s vs %s", wf.Name, name)
	}

	path, err := s.path(name)
	if err != nil {
		return workflow.Workflow{}, err
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return workflow.Workflow{}, err
	}

	return wf, nil
}

func (s *Store) ensureDir() error {
	if strings.TrimSpace(s.Dir) == "" {
		return fmt.Errorf("store dir is empty")
	}
	return os.MkdirAll(s.Dir, 0o755)
}

func (s *Store) path(name string) (string, error) {
	safe, err := sanitizeName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, safe+".yaml"), nil
}

func workflowNameFromFile(filename string) (string, bool) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".yaml", ".yml":
		return base, true
	default:
		return "", false
	}
}

func sanitizeName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("workflow name is empty")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid workflow name %q", name)
	}
	return trimmed, nil
}
