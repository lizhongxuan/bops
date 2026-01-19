package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"bops/internal/validationenv"
	"bops/internal/validationrun"
	"bops/internal/workflow"
)

type validationEnvListResponse struct {
	Items []validationenv.Summary `json:"items"`
	Total int                     `json:"total"`
}

type validationEnvRequest struct {
	Name         string                `json:"name"`
	Type         validationenv.EnvType `json:"type"`
	Description  string                `json:"description"`
	Labels       map[string]string     `json:"labels"`
	Image        string                `json:"image"`
	Host         string                `json:"host"`
	User         string                `json:"user"`
	SSHKey       string                `json:"ssh_key"`
	AgentAddress string                `json:"agent_address"`
}

type validationRunRequest struct {
	Env  string `json:"env"`
	YAML string `json:"yaml"`
}

type validationRunResponse struct {
	Status string `json:"status"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	Code   int    `json:"code"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) handleValidationEnvs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.validationStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	if search != "" {
		filtered := items[:0]
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Name), search) ||
				strings.Contains(strings.ToLower(item.Description), search) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	writeJSON(w, http.StatusOK, validationEnvListResponse{Items: items, Total: len(items)})
}

func (s *Server) handleValidationEnv(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/validation-envs/")
	name := strings.Trim(path, "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "validation env name is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		env, _, err := s.validationStore.Get(name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, env)
	case http.MethodPut:
		body, err := readBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var req validationEnvRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			req.Name = name
		}
		env, err := s.validationStore.Put(name, validationenv.ValidationEnv{
			Name:         req.Name,
			Type:         req.Type,
			Description:  req.Description,
			Labels:       req.Labels,
			Image:        req.Image,
			Host:         req.Host,
			User:         req.User,
			SSHKey:       req.SSHKey,
			AgentAddress: req.AgentAddress,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, env)
	case http.MethodDelete:
		if err := s.validationStore.Delete(name); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleValidationRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req validationRunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.Env) == "" {
		writeError(w, http.StatusBadRequest, "env is required")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, http.StatusBadRequest, "yaml is required")
		return
	}

	wf, err := workflow.Load([]byte(req.YAML))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := wf.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	env, _, err := s.validationStore.Get(req.Env)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	result, runErr := validationrun.Run(r.Context(), env, req.YAML)
	resp := validationRunResponse{
		Status: result.Status,
		Stdout: result.Stdout,
		Stderr: result.Stderr,
		Code:   result.Code,
	}
	if runErr != nil {
		resp.Status = "failed"
		resp.Error = runErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}
