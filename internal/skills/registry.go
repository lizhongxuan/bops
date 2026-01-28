package skills

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type RegisteredSkill struct {
	Name      string
	Version   string
	SourceDir string
	LoadedAt  time.Time
	Skill     *LoadedSkill
	Err       error
}

type Registry struct {
	mu     sync.RWMutex
	items  map[string]RegisteredSkill
	policy ConflictPolicy
	loader *Loader
}

type RegistryOption func(*Registry)

func WithConflictPolicy(policy ConflictPolicy) RegistryOption {
	return func(r *Registry) {
		if policy != "" {
			r.policy = policy
		}
	}
}

func NewRegistry(loader *Loader, opts ...RegistryOption) *Registry {
	r := &Registry{
		items:  make(map[string]RegisteredSkill),
		policy: ConflictError,
		loader: loader,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Registry) Get(name, version string) (RegisteredSkill, bool) {
	key := buildRegistryKey(name, version)
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[key]
	return item, ok
}

func (r *Registry) List() []RegisteredSkill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RegisteredSkill, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out
}

func (r *Registry) FindByName(name string) []RegisteredSkill {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RegisteredSkill, 0)
	for _, item := range r.items {
		if item.Name == trimmed {
			out = append(out, item)
		}
	}
	return out
}

func (r *Registry) Add(skill *LoadedSkill, err error) error {
	if skill == nil && err == nil {
		return fmt.Errorf("skill is nil")
	}
	var name string
	var version string
	var source string
	if skill != nil {
		name = skill.Manifest.Name
		version = skill.Manifest.Version
		source = skill.SourceDir
	}
	key := buildRegistryKey(name, version)
	item := RegisteredSkill{
		Name:      name,
		Version:   version,
		SourceDir: source,
		LoadedAt:  time.Now().UTC(),
		Skill:     skill,
		Err:       err,
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.items[key]; ok {
		switch r.policy {
		case ConflictError:
			return fmt.Errorf("skill already exists: %s", key)
		case ConflictKeepExisting:
			return nil
		case ConflictOverwrite:
			_ = existing
		default:
			return fmt.Errorf("unknown conflict policy: %s", r.policy)
		}
	}
	r.items[key] = item
	return nil
}

func (r *Registry) Refresh(skillNames []string) []RegisteredSkill {
	if r.loader == nil {
		return nil
	}

	next := make(map[string]RegisteredSkill)
	results := make([]RegisteredSkill, 0, len(skillNames))
	for _, name := range skillNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		loaded, err := r.loader.Load(trimmed)
		var entry RegisteredSkill
		if loaded != nil {
			entry = RegisteredSkill{
				Name:      loaded.Manifest.Name,
				Version:   loaded.Manifest.Version,
				SourceDir: loaded.SourceDir,
				LoadedAt:  time.Now().UTC(),
				Skill:     loaded,
				Err:       err,
			}
		} else {
			entry = RegisteredSkill{
				Name:     trimmed,
				Version:  "",
				LoadedAt: time.Now().UTC(),
				Skill:    nil,
				Err:      err,
			}
		}
		results = append(results, entry)
		key := buildRegistryKey(entry.Name, entry.Version)
		next[key] = entry
	}

	r.mu.Lock()
	r.items = next
	r.mu.Unlock()

	return results
}

func buildRegistryKey(name, version string) string {
	trimmed := strings.TrimSpace(name)
	ver := strings.TrimSpace(version)
	if trimmed == "" {
		return "unknown"
	}
	if ver == "" {
		return trimmed
	}
	return fmt.Sprintf("%s@%s", trimmed, ver)
}
