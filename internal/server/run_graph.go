package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"bops/internal/logging"
	"bops/internal/workbench"
	"bops/internal/workflow"
	"go.uber.org/zap"
)

type runGraphRequest struct {
	Graph  workbench.Graph `json:"graph"`
	Inputs map[string]any  `json:"inputs,omitempty"`
}

func (s *Server) handleRunGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req runGraphRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if len(req.Graph.Nodes) == 0 {
		writeError(w, r, http.StatusBadRequest, "graph is required")
		return
	}
	if strings.TrimSpace(req.Graph.Version) == "" {
		req.Graph.Version = "v1"
	}
	wf := workflow.Workflow{
		Name:    "workbench-graph",
		Version: req.Graph.Version,
	}

	runID, runCtx, err := s.runs.StartRun(context.Background(), wf)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	exec := workbench.GraphExecutor{AI: s.aiClient}
	logging.L().Info("graph run request",
		zap.String("run_id", runID),
		zap.Int("nodes", len(req.Graph.Nodes)),
		zap.Int("edges", len(req.Graph.Edges)),
	)

	go func() {
		recorder := s.runs.Recorder(runID)
		_, err := exec.Run(runCtx, req.Graph, req.Inputs, recorder)
		if err != nil {
			logging.L().Error("graph run failed", zap.String("run_id", runID), zap.Error(err))
		} else {
			logging.L().Info("graph run done", zap.String("run_id", runID))
		}
		_ = s.runs.FinishRun(runID, err)
	}()

	writeJSON(w, http.StatusOK, runResponse{RunID: runID, Status: "running"})
}
