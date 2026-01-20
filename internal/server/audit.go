package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type validationAuditEntry struct {
	Time      string `json:"time"`
	Source    string `json:"source"`
	Workflow  string `json:"workflow,omitempty"`
	Env       string `json:"env"`
	EnvType   string `json:"env_type"`
	Status    string `json:"status"`
	Code      int    `json:"code,omitempty"`
	Error     string `json:"error,omitempty"`
	YAMLHash  string `json:"yaml_hash,omitempty"`
	StepCount int    `json:"step_count,omitempty"`
}

func (s *Server) recordValidationAudit(entry validationAuditEntry) {
	if s.auditLogPath == "" {
		return
	}
	if entry.Time == "" {
		entry.Time = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.auditLogPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(s.auditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(data, '\n'))
}

func hashYAML(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
