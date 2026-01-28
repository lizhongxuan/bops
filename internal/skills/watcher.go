package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bops/internal/config"
)

type Watcher struct {
	ConfigPath string
	SkillRoot  string
	Registry   *Registry
	Loader     *Loader
	Interval   time.Duration

	lastConfigMod time.Time
	lastSkillMod  time.Time
}

func NewWatcher(configPath string, registry *Registry, loader *Loader) *Watcher {
	return &Watcher{
		ConfigPath: configPath,
		Registry:   registry,
		Loader:     loader,
		Interval:   5 * time.Second,
	}
}

func (w *Watcher) Start(ctx context.Context) error {
	if w.Registry == nil {
		return fmt.Errorf("registry is required")
	}
	if w.Loader == nil {
		return fmt.Errorf("loader is required")
	}
	if w.Interval <= 0 {
		w.Interval = 5 * time.Second
	}

	if err := w.Reload(); err != nil {
		return err
	}

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if w.shouldReload() {
				_ = w.Reload()
			}
		}
	}
}

func (w *Watcher) Reload() error {
	cfg, err := config.Load(w.ConfigPath)
	if err != nil {
		return err
	}
	root := w.SkillRoot
	if root == "" {
		root = w.Loader.Root
	}
	if root == "" {
		root = DefaultRoot
	}
	w.Loader.Root = root
	w.Registry.Refresh(collectSkills(cfg))
	w.lastConfigMod = fileModTime(config.ResolvePath(w.ConfigPath))
	w.lastSkillMod = latestModTime(root)
	return nil
}

func (w *Watcher) shouldReload() bool {
	configPath := config.ResolvePath(w.ConfigPath)
	configMod := fileModTime(configPath)
	if !configMod.IsZero() && configMod.After(w.lastConfigMod) {
		return true
	}

	root := w.SkillRoot
	if root == "" {
		root = w.Loader.Root
	}
	if root == "" {
		root = DefaultRoot
	}
	skillMod := latestModTime(root)
	if !skillMod.IsZero() && skillMod.After(w.lastSkillMod) {
		return true
	}

	return false
}

func collectSkills(cfg config.Config) []string {
	unique := make(map[string]struct{})
	add := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			unique[trimmed] = struct{}{}
		}
	}
	for _, skill := range cfg.ClaudeSkills {
		add(skill)
	}
	for _, agent := range cfg.Agents {
		for _, skill := range agent.Skills {
			add(skill)
		}
	}
	out := make([]string, 0, len(unique))
	for name := range unique {
		out = append(out, name)
	}
	return out
}

func fileModTime(path string) time.Time {
	if path == "" {
		return time.Time{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func latestModTime(root string) time.Time {
	if strings.TrimSpace(root) == "" {
		return time.Time{}
	}
	var latest time.Time
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}
