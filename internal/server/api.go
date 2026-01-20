package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bops/internal/core"
	"bops/internal/engine"
	"bops/internal/envstore"
	"bops/internal/report"
	"bops/internal/workflow"
	"bops/internal/workflowstore"
)

type listResponse struct {
	Items []workflowstore.Summary `json:"items"`
	Total int                     `json:"total"`
}

type envListResponse struct {
	Items []envstore.Summary `json:"items"`
	Total int                `json:"total"`
}

type workflowResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	YAML        string `json:"yaml"`
}

type validateRequest struct {
	YAML string `json:"yaml"`
}

type envPackageRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Env         map[string]string `json:"env"`
}

type envPackageResponse struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Env         map[string]string `json:"env"`
}

type runResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

type runListResponse struct {
	Items []report.Summary `json:"items"`
	Total int              `json:"total"`
}

type validateResponse struct {
	OK     bool     `json:"ok"`
	Issues []string `json:"issues,omitempty"`
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	s.mux.HandleFunc("/api/workflows", s.handleWorkflows)
	s.mux.HandleFunc("/api/workflows/", s.handleWorkflow)
	s.mux.HandleFunc("/api/envs", s.handleEnvPackages)
	s.mux.HandleFunc("/api/envs/", s.handleEnvPackage)
	s.mux.HandleFunc("/api/validation-envs", s.handleValidationEnvs)
	s.mux.HandleFunc("/api/validation-envs/", s.handleValidationEnv)
	s.mux.HandleFunc("/api/validation-runs", s.handleValidationRun)
	s.mux.HandleFunc("/api/scripts", s.handleScripts)
	s.mux.HandleFunc("/api/scripts/", s.handleScript)
	s.mux.HandleFunc("/api/ai/chat/sessions", s.handleAIChatSessions)
	s.mux.HandleFunc("/api/ai/chat/sessions/", s.handleAIChatSession)
	s.mux.HandleFunc("/api/ai/workflow/generate", s.handleAIWorkflowGenerate)
	s.mux.HandleFunc("/api/ai/workflow/fix", s.handleAIWorkflowFix)
	s.mux.HandleFunc("/api/ai/workflow/validate", s.handleAIWorkflowValidate)
	s.mux.HandleFunc("/api/ai/workflow/execute", s.handleAIWorkflowExecute)
	s.mux.HandleFunc("/api/ai/workflow/summary", s.handleAIWorkflowSummary)
	s.mux.HandleFunc("/api/ai/workflow/stream", s.handleAIWorkflowStream)
	s.mux.HandleFunc("/api/runs", s.handleRuns)
	s.mux.HandleFunc("/api/runs/", s.handleRun)

	if strings.TrimSpace(s.StaticDir) != "" {
		s.mux.Handle("/", spaHandler(s.StaticDir))
	}
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.store.List()
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

	writeJSON(w, http.StatusOK, listResponse{
		Items: items,
		Total: len(items),
	})
}

func (s *Server) handleEnvPackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.envStore.List()
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

	writeJSON(w, http.StatusOK, envListResponse{
		Items: items,
		Total: len(items),
	})
}

func (s *Server) handleEnvPackage(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/envs/")
	name := strings.Trim(path, "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "env package name is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		pkg, _, err := s.envStore.Get(name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, envPackageResponse{
			Name:        pkg.Name,
			Description: pkg.Description,
			Env:         pkg.Env,
		})
	case http.MethodPut:
		body, err := readBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var req envPackageRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			req.Name = name
		}
		pkg, err := s.envStore.Put(name, envstore.Package{
			Name:        req.Name,
			Description: req.Description,
			Env:         req.Env,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, envPackageResponse{
			Name:        pkg.Name,
			Description: pkg.Description,
			Env:         pkg.Env,
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	if path == "" {
		writeError(w, http.StatusNotFound, "workflow name is required")
		return
	}

	if strings.HasSuffix(path, "/validate") {
		s.handleValidate(w, r, strings.TrimSuffix(path, "/validate"))
		return
	}
	if strings.HasSuffix(path, "/plan") {
		s.handlePlan(w, r, strings.TrimSuffix(path, "/plan"))
		return
	}
	if strings.HasSuffix(path, "/apply") {
		s.handleApply(w, r, strings.TrimSuffix(path, "/apply"))
		return
	}
	if strings.HasSuffix(path, "/runs") {
		s.handleWorkflowRuns(w, r, strings.TrimSuffix(path, "/runs"))
		return
	}

	name := strings.Trim(path, "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "workflow name is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		wf, raw, err := s.store.Get(name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, workflowResponse{
			Name:        name,
			Description: wf.Description,
			YAML:        string(raw),
		})
	case http.MethodPut:
		body, err := readBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var req validateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(req.YAML) == "" {
			writeError(w, http.StatusBadRequest, "yaml is required")
			return
		}
		if _, err := s.store.Put(name, []byte(req.YAML)); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req validateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, http.StatusBadRequest, "yaml is required")
		return
	}

	wf, err := workflow.Load([]byte(req.YAML))
	if err != nil {
		writeJSON(w, http.StatusOK, validateResponse{OK: false, Issues: []string{err.Error()}})
		return
	}
	if err := wf.Validate(); err != nil {
		if validationErr, ok := err.(*workflow.ValidationError); ok {
			writeJSON(w, http.StatusOK, validateResponse{OK: false, Issues: validationErr.Issues})
			return
		}
		writeJSON(w, http.StatusOK, validateResponse{OK: false, Issues: []string{err.Error()}})
		return
	}

	writeJSON(w, http.StatusOK, validateResponse{OK: true})
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name = strings.Trim(name, "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "workflow name is required")
		return
	}

	wf, _, err := s.store.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	envMap, err := s.loadEnvPackages(wf.EnvPackages)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	applyEnvToWorkflow(&wf, envMap)

	plan, err := s.engine.Plan(r.Context(), wf)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name = strings.Trim(name, "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "workflow name is required")
		return
	}

	wf, _, err := s.store.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	envMap, err := s.loadEnvPackages(wf.EnvPackages)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	applyEnvToWorkflow(&wf, envMap)

	runID, runCtx, err := s.runs.StartRun(context.Background(), wf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go func() {
		recorder := s.runs.Recorder(runID)
		ctx := engine.WithRecorder(runCtx, recorder)
		ctx = engine.WithEnv(ctx, envMap)
		err := s.engine.Apply(ctx, wf)
		_ = s.runs.FinishRun(runID, err)
	}()

	writeJSON(w, http.StatusOK, runResponse{RunID: runID, Status: "running"})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.listRuns(r, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, runListResponse{
		Items: items,
		Total: len(items),
	})
}

func (s *Server) handleWorkflowRuns(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name = strings.Trim(name, "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "workflow name is required")
		return
	}

	items, err := s.listRuns(r, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, runListResponse{
		Items: items,
		Total: len(items),
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	runID = strings.Trim(runID, "/")
	if runID == "" {
		writeError(w, http.StatusNotFound, "run id is required")
		return
	}
	if strings.HasSuffix(runID, "/stream") {
		s.handleRunStream(w, r, strings.TrimSuffix(runID, "/stream"))
		return
	}
	if strings.HasSuffix(runID, "/stop") {
		s.handleRunStop(w, r, strings.TrimSuffix(runID, "/stop"))
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	run, ok, err := s.runs.GetRun(runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"run": run, "steps": run.Steps})
}

func (s *Server) handleRunStop(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	runID = strings.Trim(runID, "/")
	if runID == "" {
		writeError(w, http.StatusNotFound, "run id is required")
		return
	}

	if err := s.runs.StopRun(runID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRunStream(w http.ResponseWriter, r *http.Request, runID string) {
	runID = strings.Trim(runID, "/")
	if runID == "" {
		writeError(w, http.StatusNotFound, "run id is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sub := s.bus.Subscribe(128)
	defer sub.Cancel()

	fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-sub.C:
			if !ok {
				return
			}
			if evt.RunID != runID {
				continue
			}

			payload, err := json.Marshal(evt)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName(evt), payload)
			flusher.Flush()
		}
	}
}

func eventName(evt core.Event) string {
	if evt.Type == "" {
		return "message"
	}
	return string(evt.Type)
}

func (s *Server) listRuns(r *http.Request, workflowName string) ([]report.Summary, error) {
	runs, err := s.runs.ListRuns()
	if err != nil {
		return nil, err
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	from, _ := parseTime(r.URL.Query().Get("from"))
	to, _ := parseTime(r.URL.Query().Get("to"))

	items := make([]report.Summary, 0, len(runs))
	for _, run := range runs {
		if workflowName != "" && run.WorkflowName != workflowName {
			continue
		}

		if !from.IsZero() && run.StartedAt.Before(from) {
			continue
		}
		if !to.IsZero() && run.StartedAt.After(to) {
			continue
		}

		summary := report.Summarize(run)
		if status != "" && summary.Status != status {
			continue
		}

		items = append(items, summary)
	}

	return items, nil
}

func parseTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func (s *Server) loadEnvPackages(names []string) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}

	env := map[string]string{}
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		pkg, _, err := s.envStore.Get(trimmed)
		if err != nil {
			return nil, fmt.Errorf("env package %q not found: %w", trimmed, err)
		}
		for k, v := range pkg.Env {
			env[k] = v
		}
	}
	return env, nil
}

func applyEnvToWorkflow(wf *workflow.Workflow, env map[string]string) {
	if wf == nil || len(env) == 0 {
		return
	}
	if wf.Vars == nil {
		wf.Vars = map[string]any{}
	}
	envAny := map[string]any{}
	for k, v := range env {
		envAny[k] = v
	}
	wf.Vars["env"] = envAny
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	const maxSize = 4 << 20
	return io.ReadAll(io.LimitReader(r.Body, maxSize))
}
