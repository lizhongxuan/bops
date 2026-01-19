package scriptstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Summary struct {
	Name        string    `json:"name"`
	Language    string    `json:"language"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags,omitempty"`
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
		name, ok := scriptNameFromFile(entry.Name())
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		summary := Summary{
			Name:      name,
			UpdatedAt: info.ModTime().UTC(),
		}

		if data, err := os.ReadFile(filepath.Join(s.Dir, entry.Name())); err == nil {
			var script Script
			if err := yaml.Unmarshal(data, &script); err == nil {
				if script.Name == "" {
					script.Name = name
				}
				summary.Name = script.Name
				summary.Language = script.Language
				summary.Description = script.Description
				summary.Tags = append([]string{}, script.Tags...)
			}
		}

		items = append(items, summary)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	return items, nil
}

func (s *Store) Get(name string) (Script, []byte, error) {
	path, err := s.path(name)
	if err != nil {
		return Script{}, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Script{}, nil, err
	}

	var script Script
	if err := yaml.Unmarshal(data, &script); err != nil {
		return Script{}, nil, err
	}
	if script.Name == "" {
		script.Name = name
	}

	return script, data, nil
}

func (s *Store) Put(name string, script Script) (Script, error) {
	if err := s.ensureDir(); err != nil {
		return Script{}, err
	}

	if strings.TrimSpace(name) == "" {
		name = script.Name
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Script{}, fmt.Errorf("script name is required")
	}
	if script.Name != "" && script.Name != name {
		return Script{}, fmt.Errorf("script name mismatch: %s vs %s", script.Name, name)
	}
	script.Name = name
	lang := strings.TrimSpace(script.Language)
	if lang == "" {
		return Script{}, fmt.Errorf("script language is required")
	}
	switch lang {
	case "shell", "python":
	default:
		return Script{}, fmt.Errorf("unsupported script language: %s", lang)
	}
	script.Language = lang

	path, err := s.path(name)
	if err != nil {
		return Script{}, err
	}

	raw, err := yaml.Marshal(script)
	if err != nil {
		return Script{}, err
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return Script{}, err
	}

	return script, nil
}

func (s *Store) Delete(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
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

func scriptNameFromFile(filename string) (string, bool) {
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
		return "", fmt.Errorf("script name is empty")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid script name %q", name)
	}
	return trimmed, nil
}
