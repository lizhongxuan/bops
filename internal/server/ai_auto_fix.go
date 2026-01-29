package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"bops/internal/aiworkflow"
	"bops/internal/logging"
	"go.uber.org/zap"
)

type autoFixRequest struct {
	YAML          string         `json:"yaml"`
	ValidationEnv string         `json:"validation_env,omitempty"`
	MaxRetries    int            `json:"max_retries,omitempty"`
	Context       map[string]any `json:"context,omitempty"`
}

type autoFixResponse struct {
	YAML        string   `json:"yaml"`
	Issues      []string `json:"issues,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	DraftID     string   `json:"draft_id,omitempty"`
	RiskLevel   string   `json:"risk_level,omitempty"`
	NeedsReview bool     `json:"needs_review,omitempty"`
	Diffs       []string `json:"diffs,omitempty"`
}

func (s *Server) handleAIWorkflowAutoFixRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.aiWorkflow == nil {
		writeError(w, r, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "streaming unsupported")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req autoFixRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}

	ctx := r.Context()
	maxRetries := req.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}

	logging.L().Info("ai workflow auto-fix request",
		zap.Int("yaml_len", len(req.YAML)),
		zap.Int("max_retries", maxRetries),
		zap.String("validation_env", strings.TrimSpace(req.ValidationEnv)),
	)

	writeSSE(w, "status", map[string]any{"status": "start", "message": "auto-fix start"})
	flusher.Flush()

	state, err := s.aiWorkflow.RunFix(ctx, req.YAML, nil, aiworkflow.RunOptions{
		SystemPrompt: s.systemPrompt(s.buildContextText(req.Context)),
		ContextText:  s.buildContextText(req.Context),
		SkipExecute:  true,
		MaxRetries:   maxRetries,
		EventSink: func(evt aiworkflow.Event) {
			writeSSE(w, "status", map[string]any{
				"node":    evt.Node,
				"status":  evt.Status,
				"message": evt.Message,
			})
			flusher.Flush()
		},
	})
	if err != nil {
		logging.L().Error("ai workflow auto-fix failed", zap.Error(err))
		writeSSE(w, "error", map[string]any{"error": err.Error()})
		flusher.Flush()
		return
	}

	draftID := s.saveAIDraft("", "Auto Fix", "", state)
	logging.L().Info("ai workflow auto-fix done",
		zap.String("draft_id", draftID),
		zap.Int("yaml_len", len(state.YAML)),
		zap.Int("issues", len(state.Issues)),
		zap.Int("diffs", len(state.AutoFixDiffs)),
		zap.Bool("needs_review", state.NeedsReview),
	)
	writeSSE(w, "result", autoFixResponse{
		YAML:        state.YAML,
		Issues:      state.Issues,
		Summary:     state.Summary,
		DraftID:     draftID,
		RiskLevel:   string(state.RiskLevel),
		NeedsReview: state.NeedsReview,
		Diffs:       state.AutoFixDiffs,
	})
	flusher.Flush()
	// keep connection briefly for frontend to receive final data
	time.Sleep(10 * time.Millisecond)
}
