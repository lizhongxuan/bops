package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"bops/internal/logging"
	"bops/internal/workbench"
	"go.uber.org/zap"
)

type graphFromYAMLRequest struct {
	YAML string `json:"yaml"`
}

type graphFromYAMLResponse struct {
	Graph any `json:"graph"`
}

func (s *Server) handleAIWorkflowGraphFromYAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	var req graphFromYAMLRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	yamlText := strings.TrimSpace(req.YAML)
	if yamlText == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}
	logging.L().Debug("ai workflow graph-from-yaml request", zap.Int("yaml_len", len(yamlText)))
	graph, err := workbench.GraphFromYAML(yamlText)
	if err != nil {
		logging.L().Warn("ai workflow graph-from-yaml failed", zap.Error(err))
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	logging.L().Debug("ai workflow graph-from-yaml done",
		zap.Int("nodes", len(graph.Nodes)),
		zap.Int("edges", len(graph.Edges)),
	)
	writeJSON(w, http.StatusOK, graphFromYAMLResponse{Graph: graph})
}
