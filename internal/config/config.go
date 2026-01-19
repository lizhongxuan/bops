package config

import (
	"encoding/json"
	"os"
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

// Load reads config from a JSON file. Missing file falls back to defaults.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		if env := os.Getenv("BOPS_CONFIG"); env != "" {
			path = env
		} else {
			path = "bops.json"
		}
	}

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
