package stepsstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bops/runner/logging"
	"bops/runner/workflow"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Summary struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type StepsDoc struct {
	Version       string            `json:"version" yaml:"version"`
	Name          string            `json:"name" yaml:"name"`
	Description   string            `json:"description,omitempty" yaml:"description,omitempty"`
	EnvPackages   []string          `json:"env_packages,omitempty" yaml:"env_packages,omitempty"`
	ValidationEnv string            `json:"validation_env,omitempty" yaml:"validation_env,omitempty"`
	Vars          map[string]any    `json:"vars,omitempty" yaml:"vars,omitempty"`
	Plan          workflow.Plan     `json:"plan,omitempty" yaml:"plan,omitempty"`
	Steps         []workflow.Step   `json:"steps" yaml:"steps"`
}

type InventoryDoc struct {
	Inventory workflow.Inventory `json:"inventory" yaml:"inventory"`
}

type Store struct {
	Dir string
}

func New(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) List() ([]Summary, error) {
	logging.L().Debug("steps list", zap.String("dir", s.Dir))
	if err := s.ensureDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}

	items := make([]Summary, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			seen[name] = struct{}{}
		} else if legacyName, ok := legacyNameFromFile(entry.Name()); ok {
			if _, done := seen[legacyName]; done {
				continue
			}
			if err := s.migrateLegacy(legacyName); err == nil {
				seen[legacyName] = struct{}{}
			}
		}
	}

	for name := range seen {
		stepsPath := filepath.Join(s.Dir, name, "steps.yaml")
		stepsData, err := os.ReadFile(stepsPath)
		if err != nil {
			continue
		}
		var doc StepsDoc
		if err := yaml.Unmarshal(stepsData, &doc); err != nil {
			continue
		}

		info, err := os.Stat(stepsPath)
		if err != nil {
			continue
		}
		updated := info.ModTime()
		if invInfo, err := os.Stat(filepath.Join(s.Dir, name, "inventory.yaml")); err == nil {
			if invInfo.ModTime().After(updated) {
				updated = invInfo.ModTime()
			}
		}

		items = append(items, Summary{
			Name:        name,
			Description: strings.TrimSpace(doc.Description),
			UpdatedAt:   updated,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	logging.L().Debug("steps list done", zap.Int("count", len(items)))
	return items, nil
}

func (s *Store) GetSteps(name string) (StepsDoc, []byte, error) {
	logging.L().Debug("steps get", zap.String("name", name))
	path, err := s.stepsPath(name)
	if err != nil {
		return StepsDoc{}, nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if err := s.migrateLegacy(name); err == nil {
				// retry after migration
				if _, err := os.Stat(path); err != nil {
					return StepsDoc{}, nil, err
				}
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StepsDoc{}, nil, err
	}
	var doc StepsDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return StepsDoc{}, nil, err
	}
	if strings.TrimSpace(doc.Name) == "" {
		doc.Name = name
	}
	return doc, data, nil
}

func (s *Store) PutSteps(name string, raw []byte) (StepsDoc, error) {
	logging.L().Debug("steps put", zap.String("name", name), zap.Int("yaml_len", len(raw)))
	if err := s.ensureDir(); err != nil {
		return StepsDoc{}, err
	}
	var doc StepsDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return StepsDoc{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = strings.TrimSpace(doc.Name)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return StepsDoc{}, fmt.Errorf("workflow name is required")
	}
	if doc.Name != "" && doc.Name != name {
		return StepsDoc{}, fmt.Errorf("workflow name mismatch: %s vs %s", doc.Name, name)
	}
	doc.Name = name

	path, err := s.stepsPath(name)
	if err != nil {
		return StepsDoc{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return StepsDoc{}, err
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return StepsDoc{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return StepsDoc{}, err
	}
	logging.L().Debug("steps put done", zap.String("name", name))
	return doc, nil
}

func (s *Store) GetInventory(name string) (InventoryDoc, []byte, error) {
	logging.L().Debug("inventory get", zap.String("name", name))
	path, err := s.inventoryPath(name)
	if err != nil {
		return InventoryDoc{}, nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if err := s.migrateLegacy(name); err == nil {
				if _, err := os.Stat(path); err != nil {
					return InventoryDoc{}, nil, err
				}
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return InventoryDoc{}, nil, err
	}
	var doc InventoryDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return InventoryDoc{}, nil, err
	}
	return doc, data, nil
}

func (s *Store) PutInventory(name string, raw []byte) (InventoryDoc, error) {
	logging.L().Debug("inventory put", zap.String("name", name), zap.Int("yaml_len", len(raw)))
	if err := s.ensureDir(); err != nil {
		return InventoryDoc{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return InventoryDoc{}, fmt.Errorf("workflow name is required")
	}
	var doc InventoryDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return InventoryDoc{}, err
	}
	path, err := s.inventoryPath(name)
	if err != nil {
		return InventoryDoc{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return InventoryDoc{}, err
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return InventoryDoc{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return InventoryDoc{}, err
	}
	logging.L().Debug("inventory put done", zap.String("name", name))
	return doc, nil
}

func (s *Store) LoadWorkflow(name string) (workflow.Workflow, error) {
	steps, _, err := s.GetSteps(name)
	if err != nil {
		return workflow.Workflow{}, err
	}
	invDoc, _, err := s.GetInventory(name)
	if err != nil {
		if os.IsNotExist(err) {
			invDoc = InventoryDoc{}
		} else {
			return workflow.Workflow{}, err
		}
	}
	return BuildWorkflow(name, steps, invDoc), nil
}

func SplitWorkflow(wf workflow.Workflow, name string) (StepsDoc, InventoryDoc) {
	stepsDoc := StepsDoc{
		Version:       wf.Version,
		Name:          wf.Name,
		Description:   wf.Description,
		EnvPackages:   wf.EnvPackages,
		ValidationEnv: wf.ValidationEnv,
		Vars:          wf.Vars,
		Plan:          wf.Plan,
		Steps:         wf.Steps,
	}
	if strings.TrimSpace(stepsDoc.Name) == "" {
		stepsDoc.Name = name
	}
	return stepsDoc, InventoryDoc{Inventory: wf.Inventory}
}

func BuildWorkflow(name string, steps StepsDoc, inv InventoryDoc) workflow.Workflow {
	version := strings.TrimSpace(steps.Version)
	if version == "" {
		version = "v0.1"
	}
	planMode := "manual-approve"
	planStrategy := "sequential"
	if strings.TrimSpace(steps.Plan.Mode) != "" {
		planMode = steps.Plan.Mode
	}
	if strings.TrimSpace(steps.Plan.Strategy) != "" {
		planStrategy = steps.Plan.Strategy
	}
	wf := workflow.Workflow{
		Version:     version,
		Name:        name,
		Description: strings.TrimSpace(steps.Description),
		EnvPackages: steps.EnvPackages,
		ValidationEnv: steps.ValidationEnv,
		Vars:        steps.Vars,
		Inventory:   inv.Inventory,
		Plan: workflow.Plan{
			Mode:     planMode,
			Strategy: planStrategy,
		},
		Steps: steps.Steps,
	}
	return wf
}

func (s *Store) ensureDir() error {
	if strings.TrimSpace(s.Dir) == "" {
		return fmt.Errorf("store dir is empty")
	}
	return os.MkdirAll(s.Dir, 0o755)
}

func (s *Store) workflowDir(name string) (string, error) {
	safe, err := sanitizeName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, safe), nil
}

func (s *Store) legacyPath(name string) (string, error) {
	safe, err := sanitizeName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, safe+".yaml"), nil
}

func legacyNameFromFile(filename string) (string, bool) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".yaml", ".yml":
		return base, true
	default:
		return "", false
	}
}

func (s *Store) migrateLegacy(name string) error {
	legacyPath, err := s.legacyPath(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return err
	}
	wf, err := workflow.Load(data)
	if err != nil {
		return err
	}
	stepsDoc, invDoc := SplitWorkflow(wf, name)
	stepsRaw, err := yaml.Marshal(stepsDoc)
	if err != nil {
		return err
	}
	if _, err := s.PutSteps(name, stepsRaw); err != nil {
		return err
	}
	invRaw, err := yaml.Marshal(invDoc)
	if err != nil {
		return err
	}
	if _, err := s.PutInventory(name, invRaw); err != nil {
		return err
	}
	if err := os.Remove(legacyPath); err != nil {
		return err
	}
	return nil
}

func (s *Store) stepsPath(name string) (string, error) {
	dir, err := s.workflowDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "steps.yaml"), nil
}

func (s *Store) inventoryPath(name string) (string, error) {
	dir, err := s.workflowDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "inventory.yaml"), nil
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
