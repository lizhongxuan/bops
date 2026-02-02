package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config defines process-level settings loaded from a JSON file.
type Config struct {
	LogLevel           string        `json:"log_level"`
	LogFormat          string        `json:"log_format"`
	DataDir            string        `json:"data_dir"`
	StatePath          string        `json:"state_path"`
	ServerListen       string        `json:"server_listen"`
	AgentListen        string        `json:"agent_listen"`
	StaticDir          string        `json:"static_dir"`
	CORSOrigins        []string      `json:"cors_origins"`
	AIProvider         string        `json:"ai_provider"`
	AIApiKey           string        `json:"ai_api_key"`
	AIBaseURL          string        `json:"ai_base_url"`
	AIModel            string        `json:"ai_model"`
	AIPlannerModel     string        `json:"ai_planner_model"`
	AIExecutorModel    string        `json:"ai_executor_model"`
	ClaudeSkills       []string      `json:"claude_skills"`
	Agents             []AgentConfig `json:"agents"`
	DefaultAgent       string        `json:"default_agent"`
	DefaultAgents      []string      `json:"default_agents"`
	ToolConflictPolicy string        `json:"tool_conflict_policy"`
}

type AgentConfig struct {
	Name   string   `json:"name"`
	Role   string   `json:"role,omitempty"`
	Model  string   `json:"model"`
	Skills []string `json:"skills"`
}

// DefaultConfig returns a baseline configuration.
func DefaultConfig() Config {
	return Config{
		LogLevel:           "info",
		LogFormat:          "json",
		DataDir:            "./data",
		StatePath:          "./data/state.json",
		ServerListen:       "127.0.0.1:7070",
		AgentListen:        "127.0.0.1:7071",
		StaticDir:          "./web/dist",
		CORSOrigins:        []string{"http://127.0.0.1:5173", "http://localhost:5173"},
		AIProvider:         "",
		AIApiKey:           "",
		AIBaseURL:          "",
		AIModel:            "",
		AIPlannerModel:     "",
		AIExecutorModel:    "",
		ClaudeSkills:       nil,
		Agents:             nil,
		DefaultAgent:       "",
		DefaultAgents:      nil,
		ToolConflictPolicy: "error",
	}
}

// ResolvePath returns the config file path based on the provided path or environment defaults.
func ResolvePath(path string) string {
	if path != "" {
		return path
	}
	if env := os.Getenv("BOPS_CONFIG"); env != "" {
		return env
	}
	return "bops.json"
}

// Load reads config from a JSON file. Missing file falls back to defaults.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	path = ResolvePath(path)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}

	if err := ApplyEnvOverrides(&cfg); err != nil {
		return cfg, err
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// ApplyEnvOverrides updates config with environment values, if present.
func ApplyEnvOverrides(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if raw := os.Getenv("BOPS_CLAUDE_SKILLS"); raw != "" {
		cfg.ClaudeSkills = splitCommaList(raw)
	}
	if raw := os.Getenv("BOPS_AGENTS"); raw != "" {
		var agents []AgentConfig
		if err := json.Unmarshal([]byte(raw), &agents); err != nil {
			return err
		}
		cfg.Agents = agents
	}
	if raw := os.Getenv("BOPS_AI_PLANNER_MODEL"); raw != "" {
		cfg.AIPlannerModel = strings.TrimSpace(raw)
	}
	if raw := os.Getenv("BOPS_AI_EXECUTOR_MODEL"); raw != "" {
		cfg.AIExecutorModel = strings.TrimSpace(raw)
	}
	if raw := os.Getenv("BOPS_TOOL_CONFLICT_POLICY"); raw != "" {
		cfg.ToolConflictPolicy = raw
	}
	return nil
}

// Validate checks optional Claude skill and agent configuration.
func (cfg *Config) Validate() error {
	if cfg == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, agent := range cfg.Agents {
		name := strings.TrimSpace(agent.Name)
		if name == "" {
			return fmt.Errorf("agent name is required")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate agent name: %s", name)
		}
		seen[name] = struct{}{}
		if len(agent.Skills) == 0 {
			return fmt.Errorf("agent %s has no skills", name)
		}
	}
	if cfg.ToolConflictPolicy != "" {
		switch cfg.ToolConflictPolicy {
		case "error", "overwrite", "keep", "prefix":
		default:
			return fmt.Errorf("invalid tool_conflict_policy: %s", cfg.ToolConflictPolicy)
		}
	}
	return nil
}

func splitCommaList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// Save writes config to a JSON file using the resolved config path.
func Save(path string, cfg Config) error {
	path = ResolvePath(path)
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
