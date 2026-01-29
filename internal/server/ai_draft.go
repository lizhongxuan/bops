package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"bops/internal/aiworkflowstore"
	"bops/internal/logging"
	"bops/internal/workbench"
	"go.uber.org/zap"
)

type aiWorkflowDraftResponse struct {
	Draft aiWorkflowDraftPayload `json:"draft"`
}

type aiWorkflowDraftPayload struct {
	ID        string   `json:"id"`
	Title     string   `json:"title,omitempty"`
	Prompt    string   `json:"prompt,omitempty"`
	YAML      string   `json:"yaml,omitempty"`
	Graph     any      `json:"graph,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Issues    []string `json:"issues,omitempty"`
	RiskLevel string   `json:"risk_level,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

type aiWorkflowDraftUpdate struct {
	ID        string          `json:"id,omitempty"`
	Title     string          `json:"title,omitempty"`
	Prompt    string          `json:"prompt,omitempty"`
	YAML      string          `json:"yaml,omitempty"`
	Graph     json.RawMessage `json:"graph,omitempty"`
	Summary   string          `json:"summary,omitempty"`
	Issues    []string        `json:"issues,omitempty"`
	RiskLevel string          `json:"risk_level,omitempty"`
}

func (s *Server) handleAIWorkflowDraft(w http.ResponseWriter, r *http.Request) {
	if s.aiWorkflowStore == nil {
		writeError(w, r, http.StatusServiceUnavailable, "ai workflow store is not configured")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/ai/workflow/draft/")
	id = strings.TrimSpace(id)
	if id == "" {
		writeError(w, r, http.StatusBadRequest, "draft id is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleAIWorkflowDraftGet(w, r, id)
	case http.MethodPut:
		s.handleAIWorkflowDraftPut(w, r, id)
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAIWorkflowDraftGet(w http.ResponseWriter, r *http.Request, id string) {
	draft, _, err := s.aiWorkflowStore.Get(id)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, r, http.StatusNotFound, "draft not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	logging.L().Debug("ai draft get request", zap.String("id", id))
	if strings.TrimSpace(draft.Graph) == "" && strings.TrimSpace(draft.YAML) != "" {
		if graph, err := workbench.GraphFromYAML(draft.YAML); err == nil {
			if data, err := json.Marshal(graph); err == nil {
				draft.Graph = string(data)
				if saved, err := s.aiWorkflowStore.Save(draft); err == nil {
					draft = saved
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, aiWorkflowDraftResponse{Draft: buildDraftPayload(draft)})
}

func (s *Server) handleAIWorkflowDraftPut(w http.ResponseWriter, r *http.Request, id string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	var req aiWorkflowDraftUpdate
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ID == "" {
		req.ID = id
	}
	if req.ID != id {
		writeError(w, r, http.StatusBadRequest, "draft id mismatch")
		return
	}
	graphText := strings.TrimSpace(string(req.Graph))
	if graphText == "null" {
		graphText = ""
	}
	logging.L().Debug("ai draft save request",
		zap.String("id", req.ID),
		zap.Int("yaml_len", len(req.YAML)),
		zap.Int("graph_len", len(graphText)),
	)
	draft := aiworkflowstore.Draft{
		ID:        req.ID,
		Title:     req.Title,
		Prompt:    req.Prompt,
		YAML:      req.YAML,
		Graph:     graphText,
		Summary:   req.Summary,
		Issues:    req.Issues,
		RiskLevel: req.RiskLevel,
	}
	saved, err := s.aiWorkflowStore.Save(draft)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, aiWorkflowDraftResponse{Draft: buildDraftPayload(saved)})
}

func buildDraftPayload(draft aiworkflowstore.Draft) aiWorkflowDraftPayload {
	payload := aiWorkflowDraftPayload{
		ID:        draft.ID,
		Title:     strings.TrimSpace(draft.Title),
		Prompt:    strings.TrimSpace(draft.Prompt),
		YAML:      strings.TrimSpace(draft.YAML),
		Summary:   strings.TrimSpace(draft.Summary),
		Issues:    append([]string{}, draft.Issues...),
		RiskLevel: strings.TrimSpace(draft.RiskLevel),
	}
	if !draft.CreatedAt.IsZero() {
		payload.CreatedAt = draft.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !draft.UpdatedAt.IsZero() {
		payload.UpdatedAt = draft.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if graphPayload := parseGraphPayload(draft.Graph); graphPayload != nil {
		payload.Graph = graphPayload
	}
	return payload
}

func parseGraphPayload(graphText string) any {
	trimmed := strings.TrimSpace(graphText)
	if trimmed == "" {
		return nil
	}
	var raw any
	if err := json.Unmarshal([]byte(trimmed), &raw); err == nil {
		return raw
	}
	return trimmed
}
