package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bops/internal/config"
	"bops/internal/skills"
)

type skillInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	SourceDir   string `json:"source_dir,omitempty"`
	LoadedAt    string `json:"loaded_at,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	ErrorField  string `json:"error_field,omitempty"`
	ErrorHint   string `json:"error_hint,omitempty"`
	ErrorPath   string `json:"error_path,omitempty"`
	ToolCount   int    `json:"tool_count,omitempty"`
}

type skillsResponse struct {
	Items []skillInfo `json:"items"`
	Total int         `json:"total"`
}

type agentInfo struct {
	Name   string   `json:"name"`
	Model  string   `json:"model,omitempty"`
	Role   string   `json:"role,omitempty"`
	Skills []string `json:"skills"`
}

type agentListResponse struct {
	Items []agentInfo `json:"items"`
	Total int         `json:"total"`
}

func (s *Server) initSkills(cfg config.Config) {
	if s.skillRegistry != nil {
		return
	}
	baseDir := filepath.Dir(config.ResolvePath(s.configPath))
	root := skills.ResolveRoot(baseDir, "")
	loader := skills.NewLoader(root)
	registry := skills.NewRegistry(loader)
	s.skillLoader = loader
	s.skillRegistry = registry
	_ = s.reloadSkillsFromConfig(cfg)
}

func (s *Server) reloadSkillsFromConfig(cfg config.Config) []skills.RegisteredSkill {
	if s.skillRegistry == nil {
		return nil
	}
	return s.skillRegistry.Refresh(collectSkillRefs(cfg))
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg = cfg
	s.initSkills(cfg)

	items := []skills.RegisteredSkill{}
	if s.skillRegistry != nil {
		items = s.skillRegistry.List()
	}
	resp := buildSkillResponse(items)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSkillsReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg = cfg
	s.initSkills(cfg)
	items := s.reloadSkillsFromConfig(cfg)
	resp := buildSkillResponse(items)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg = cfg
	items := make([]agentInfo, 0, len(cfg.Agents))
	for _, agent := range cfg.Agents {
		items = append(items, agentInfo{
			Name:   strings.TrimSpace(agent.Name),
			Model:  strings.TrimSpace(agent.Model),
			Role:   strings.TrimSpace(agent.Role),
			Skills: append([]string{}, agent.Skills...),
		})
	}
	writeJSON(w, http.StatusOK, agentListResponse{Items: items, Total: len(items)})
}

func buildSkillResponse(items []skills.RegisteredSkill) skillsResponse {
	out := make([]skillInfo, 0, len(items))
	for _, item := range items {
		info := skillInfo{
			Name:      item.Name,
			Version:   item.Version,
			SourceDir: item.SourceDir,
			Status:    "loaded",
		}
		if !item.LoadedAt.IsZero() {
			info.LoadedAt = item.LoadedAt.UTC().Format(time.RFC3339)
		}
		if item.Skill != nil {
			info.Description = strings.TrimSpace(item.Skill.Manifest.Description)
			info.ToolCount = len(item.Skill.Tools)
		} else {
			info.Status = "error"
		}
		if item.Err != nil {
			info.Status = "error"
			info.Error = item.Err.Error()
			var loadErr *skills.LoadError
			if errors.As(item.Err, &loadErr) {
				if loadErr.Message != "" {
					info.Error = loadErr.Message
				}
				info.ErrorField = loadErr.Field
				info.ErrorHint = loadErr.Hint
				info.ErrorPath = loadErr.Path
			}
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Version < out[j].Version
		}
		return out[i].Name < out[j].Name
	})
	return skillsResponse{Items: out, Total: len(out)}
}

func collectSkillRefs(cfg config.Config) []string {
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
	sort.Strings(out)
	return out
}
