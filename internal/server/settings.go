package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"bops/internal/ai"
	"bops/internal/aiworkflow"
	"bops/internal/config"
)

type aiSettingsRequest struct {
	Provider *string `json:"ai_provider"`
	APIKey   *string `json:"ai_api_key"`
	BaseURL  *string `json:"ai_base_url"`
	Model    *string `json:"ai_model"`
}

type aiSettingsResponse struct {
	Provider   string `json:"ai_provider"`
	APIKeySet  bool   `json:"ai_api_key_set"`
	BaseURL    string `json:"ai_base_url"`
	Model      string `json:"ai_model"`
	Configured bool   `json:"configured"`
}

func (s *Server) handleAISettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.buildAISettingsResponse())
		return
	case http.MethodPut:
		body, err := readBody(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		var req aiSettingsRequest
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid json payload")
				return
			}
		}

		if req.Provider != nil {
			provider := strings.TrimSpace(*req.Provider)
			if provider == "none" {
				provider = ""
			}
			s.cfg.AIProvider = provider
		}
		if req.APIKey != nil {
			s.cfg.AIApiKey = strings.TrimSpace(*req.APIKey)
		}
		if req.BaseURL != nil {
			s.cfg.AIBaseURL = strings.TrimSpace(*req.BaseURL)
		}
		if req.Model != nil {
			s.cfg.AIModel = strings.TrimSpace(*req.Model)
		}

		if err := config.Save(s.configPath, s.cfg); err != nil {
			writeError(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		s.applyAIConfig()
		writeJSON(w, http.StatusOK, s.buildAISettingsResponse())
		return
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
}

func (s *Server) buildAISettingsResponse() aiSettingsResponse {
	configured := s.cfg.AIProvider != "" && s.cfg.AIApiKey != ""
	return aiSettingsResponse{
		Provider:   s.cfg.AIProvider,
		APIKeySet:  s.cfg.AIApiKey != "",
		BaseURL:    s.cfg.AIBaseURL,
		Model:      s.cfg.AIModel,
		Configured: configured,
	}
}

func (s *Server) applyAIConfig() {
	aiClient, _ := ai.NewClient(ai.Config{
		Provider: s.cfg.AIProvider,
		APIKey:   s.cfg.AIApiKey,
		BaseURL:  s.cfg.AIBaseURL,
		Model:    s.cfg.AIModel,
	})
	s.aiClient = aiClient
	if aiClient == nil {
		s.aiWorkflow = nil
		return
	}
	workflow, err := aiworkflow.New(aiworkflow.Config{
		Client:       aiClient,
		SystemPrompt: s.aiPrompt,
		MaxRetries:   2,
	})
	if err != nil {
		s.aiWorkflow = nil
		return
	}
	s.aiWorkflow = workflow
}
