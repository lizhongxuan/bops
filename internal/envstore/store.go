package envstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Package struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Env         map[string]string `json:"env" yaml:"env"`
}

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
		name, ok := packageNameFromFile(entry.Name())
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
			var pkg Package
			if err := yaml.Unmarshal(data, &pkg); err == nil {
				if strings.TrimSpace(pkg.Description) != "" {
					summary.Description = pkg.Description
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

func (s *Store) Get(name string) (Package, []byte, error) {
	path, err := s.path(name)
	if err != nil {
		return Package{}, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Package{}, nil, err
	}

	var pkg Package
	if err := yaml.Unmarshal(data, &pkg); err != nil {
		return Package{}, nil, err
	}
	if pkg.Name == "" {
		pkg.Name = name
	}

	return pkg, data, nil
}

func (s *Store) Put(name string, pkg Package) (Package, error) {
	if err := s.ensureDir(); err != nil {
		return Package{}, err
	}

	if strings.TrimSpace(name) == "" {
		name = pkg.Name
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Package{}, fmt.Errorf("env package name is required")
	}
	if pkg.Name != "" && pkg.Name != name {
		return Package{}, fmt.Errorf("env package name mismatch: %s vs %s", pkg.Name, name)
	}
	pkg.Name = name

	path, err := s.path(name)
	if err != nil {
		return Package{}, err
	}

	raw, err := yaml.Marshal(pkg)
	if err != nil {
		return Package{}, err
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return Package{}, err
	}

	return pkg, nil
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

func packageNameFromFile(filename string) (string, bool) {
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
		return "", fmt.Errorf("env package name is empty")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid env package name %q", name)
	}
	return trimmed, nil
}
