package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"bops/internal/engine"
	"bops/internal/logging"
	"bops/internal/workflow"
	"go.uber.org/zap"
)

type runWorkflowRequest struct {
	YAML string `json:"yaml"`
}

type runWorkflowResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req runWorkflowRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}

	logging.L().Info("run workflow request", zap.Int("yaml_len", len(req.YAML)))
	wf, err := workflow.Load([]byte(req.YAML))
	if err != nil {
		logging.L().Warn("run workflow load failed", zap.Error(err))
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	runID, runCtx, err := s.runs.StartRun(context.Background(), wf)
	if err != nil {
		logging.L().Error("run workflow start failed", zap.Error(err))
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	logging.L().Info("run workflow started",
		zap.String("run_id", runID),
		zap.String("workflow", wf.Name),
		zap.Int("steps", len(wf.Steps)),
	)

	go func() {
		recorder := s.runs.Recorder(runID)
		ctx := engine.WithRecorder(runCtx, recorder)
		err := s.engine.Apply(ctx, wf)
		if err != nil {
			logging.L().Error("run workflow apply failed", zap.String("run_id", runID), zap.Error(err))
		} else {
			logging.L().Info("run workflow apply done", zap.String("run_id", runID))
		}
		_ = s.runs.FinishRun(runID, err)
	}()

	writeJSON(w, http.StatusOK, runWorkflowResponse{RunID: runID, Status: "running"})
}
