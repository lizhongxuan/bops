package validationenv

import (
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

type Summary struct {
	Name        string    `json:"name"`
	Type        EnvType   `json:"type"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store struct {
	Dir string
}

func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) List() ([]Summary, error) {
	logging.L().Debug("validation env list", zap.String("dir", s.Dir))
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
		name, ok := envNameFromFile(entry.Name())
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
			var env ValidationEnv
			if err := yaml.Unmarshal(data, &env); err == nil {
				if env.Name == "" {
					env.Name = name
				}
				summary.Name = env.Name
				summary.Type = env.Type
				summary.Description = env.Description
			}
		}

		items = append(items, summary)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	logging.L().Debug("validation env list done", zap.Int("count", len(items)))
	return items, nil
}

func (s *Store) Get(name string) (ValidationEnv, []byte, error) {
	logging.L().Debug("validation env get", zap.String("name", name))
	path, err := s.path(name)
	if err != nil {
		return ValidationEnv{}, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ValidationEnv{}, nil, err
	}

	var env ValidationEnv
	if err := yaml.Unmarshal(data, &env); err != nil {
		return ValidationEnv{}, nil, err
	}
	if env.Name == "" {
		env.Name = name
	}

	logging.L().Debug("validation env get done", zap.String("name", env.Name), zap.String("type", string(env.Type)))
	return env, data, nil
}

func (s *Store) Put(name string, env ValidationEnv) (ValidationEnv, error) {
	logging.L().Debug("validation env put", zap.String("name", name), zap.String("type", string(env.Type)))
	if err := s.ensureDir(); err != nil {
		return ValidationEnv{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = env.Name
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ValidationEnv{}, fmt.Errorf("validation env name is required")
	}
	if env.Name != "" && env.Name != name {
		return ValidationEnv{}, fmt.Errorf("validation env name mismatch: %s vs %s", env.Name, name)
	}
	env.Name = name
	if err := validateEnv(env); err != nil {
		return ValidationEnv{}, err
	}

	path, err := s.path(name)
	if err != nil {
		return ValidationEnv{}, err
	}

	raw, err := yaml.Marshal(env)
	if err != nil {
		return ValidationEnv{}, err
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return ValidationEnv{}, err
	}

	logging.L().Debug("validation env put done", zap.String("name", env.Name))
	return env, nil
}

func (s *Store) Delete(name string) error {
	logging.L().Debug("validation env delete", zap.String("name", name))
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

func envNameFromFile(filename string) (string, bool) {
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
		return "", fmt.Errorf("validation env name is empty")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid validation env name %q", name)
	}
	return trimmed, nil
}

func validateEnv(env ValidationEnv) error {
	if env.Type == "" {
		return fmt.Errorf("validation env type is required")
	}
	switch env.Type {
	case EnvTypeContainer:
		if strings.TrimSpace(env.Image) == "" {
			return fmt.Errorf("container image is required")
		}
	case EnvTypeSSH:
		if strings.TrimSpace(env.Host) == "" {
			return fmt.Errorf("ssh host is required")
		}
	case EnvTypeAgent:
		if strings.TrimSpace(env.AgentAddress) == "" {
			return fmt.Errorf("agent address is required")
		}
	default:
		return fmt.Errorf("unknown validation env type: %s", env.Type)
	}
	return nil
}
