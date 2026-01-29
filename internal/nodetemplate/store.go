package nodetemplate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bops/internal/logging"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Store struct {
	Dir string
}

func New(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) List() ([]Summary, error) {
	logging.L().Debug("node template list", zap.String("dir", s.Dir))
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
		name, ok := templateNameFromFile(entry.Name())
		if !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var tpl Template
	if err := yaml.Unmarshal(data, &tpl); err != nil {
		return nil, err
	}
	if tpl.Name == "" {
		tpl.Name = name
	}
	items = append(items, Summary{
		Name:        tpl.Name,
		Category:    tpl.Category,
		Description: tpl.Description,
		Tags:        append([]string{}, tpl.Tags...),
		Node:        tpl.Node,
	})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			return items[i].Name < items[j].Name
		}
		return items[i].Category < items[j].Category
	})
	logging.L().Debug("node template list done", zap.Int("count", len(items)))
	return items, nil
}

func (s *Store) Get(name string) (Template, []byte, error) {
	logging.L().Debug("node template get", zap.String("name", name))
	path, err := s.path(name)
	if err != nil {
		return Template{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Template{}, nil, err
	}
	var tpl Template
	if err := yaml.Unmarshal(data, &tpl); err != nil {
		return Template{}, nil, err
	}
	if tpl.Name == "" {
		tpl.Name = name
	}
	logging.L().Debug("node template get done", zap.String("name", tpl.Name))
	return tpl, data, nil
}

func (s *Store) Put(name string, tpl Template) (Template, error) {
	logging.L().Debug("node template put", zap.String("name", name))
	if err := s.ensureDir(); err != nil {
		return Template{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = tpl.Name
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, fmt.Errorf("template name is required")
	}
	if tpl.Name != "" && tpl.Name != name {
		return Template{}, fmt.Errorf("template name mismatch: %s vs %s", tpl.Name, name)
	}
	if strings.TrimSpace(tpl.Category) == "" {
		return Template{}, fmt.Errorf("template category is required")
	}
	if strings.TrimSpace(tpl.Node.Type) == "" {
		return Template{}, fmt.Errorf("template node type is required")
	}
	if strings.TrimSpace(tpl.Node.Name) == "" {
		tpl.Node.Name = tpl.Name
	}
	tpl.Name = name
	path, err := s.path(name)
	if err != nil {
		return Template{}, err
	}
	data, err := yaml.Marshal(tpl)
	if err != nil {
		return Template{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Template{}, err
	}
	logging.L().Debug("node template put done", zap.String("name", tpl.Name))
	return tpl, nil
}

func (s *Store) Delete(name string) error {
	logging.L().Debug("node template delete", zap.String("name", name))
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

func templateNameFromFile(filename string) (string, bool) {
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
		return "", fmt.Errorf("template name is empty")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid template name %q", name)
	}
	return trimmed, nil
}
