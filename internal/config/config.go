package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config defines process-level settings loaded from a JSON file.
type Config struct {
	LogLevel     string   `json:"log_level"`
	LogFormat    string   `json:"log_format"`
	DataDir      string   `json:"data_dir"`
	StatePath    string   `json:"state_path"`
	ServerListen string   `json:"server_listen"`
	AgentListen  string   `json:"agent_listen"`
	StaticDir    string   `json:"static_dir"`
	CORSOrigins  []string `json:"cors_origins"`
	AIProvider   string   `json:"ai_provider"`
	AIApiKey     string   `json:"ai_api_key"`
	AIBaseURL    string   `json:"ai_base_url"`
	AIModel      string   `json:"ai_model"`
}

// DefaultConfig returns a baseline configuration.
func DefaultConfig() Config {
	return Config{
		LogLevel:     "info",
		LogFormat:    "json",
		DataDir:      "./data",
		StatePath:    "./data/state.json",
		ServerListen: "127.0.0.1:7070",
		AgentListen:  "127.0.0.1:7071",
		StaticDir:    "./web/dist",
		CORSOrigins:  []string{"http://127.0.0.1:5173", "http://localhost:5173"},
		AIProvider:   "",
		AIApiKey:     "",
		AIBaseURL:    "",
		AIModel:      "",
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

	return cfg, nil
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
