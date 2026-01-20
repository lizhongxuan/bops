package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"bops/internal/aistore"
	"bops/internal/envstore"
	"bops/internal/eventbus"
	"bops/internal/runmanager"
	"bops/internal/scheduler"
	"bops/internal/scriptstore"
	"bops/internal/state"
	"bops/internal/workflow"
	"bops/internal/workflowstore"
)

func newRunTestServer(t *testing.T) (*Server, *runmanager.Manager) {
	t.Helper()
	dir := t.TempDir()
	bus := eventbus.New()
	runs := runmanager.NewWithBus(state.NewFileStore(filepath.Join(dir, "state.json")), bus)
	srv := &Server{
		mux:         http.NewServeMux(),
		store:       workflowstore.New(filepath.Join(dir, "workflows")),
		envStore:    envstore.New(filepath.Join(dir, "envs")),
		aiStore:     aistore.New(filepath.Join(dir, "ai_sessions")),
		scriptStore: scriptstore.New(filepath.Join(dir, "scripts")),
		aiPrompt:    "test",
		runs:        runs,
		bus:         bus,
	}
	srv.routes()
	return srv, runs
}

func TestRunAPI_StatusPayload(t *testing.T) {
	srv, runs := newRunTestServer(t)

	wf := workflow.Workflow{
		Version: "v0.1",
		Name:    "demo",
		Steps: []workflow.Step{
			{
				Name:   "step-1",
				Action: "cmd.run",
				Targets: []string{
					"web",
				},
				With: map[string]any{
					"cmd": "echo hi",
				},
			},
		},
	}

	runID, _, err := runs.StartRun(context.Background(), wf)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	rec := runs.Recorder(runID)
	step := wf.Steps[0]
	target := workflow.HostSpec{Name: "web"}
	rec.StepStart(step, []workflow.HostSpec{target})
	rec.HostResult(step, target, scheduler.Result{
		Status: "failed",
		Output: map[string]any{"stderr": "boom"},
		Error:  "boom",
	})
	rec.StepFinish(step, "failed")
	_ = runs.FinishRun(runID, context.Canceled)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+runID, nil)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var resp struct {
		Run struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"run"`
		Steps []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Hosts  map[string]struct {
				Status  string                 `json:"status"`
				Message string                 `json:"message"`
				Output  map[string]interface{} `json:"output"`
			} `json:"hosts"`
		} `json:"steps"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Run.Status != "failed" {
		t.Fatalf("expected run status failed, got %q", resp.Run.Status)
	}
	if resp.Run.Message == "" {
		t.Fatalf("expected run message to be set")
	}
	if len(resp.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(resp.Steps))
	}
	if resp.Steps[0].Status != "failed" {
		t.Fatalf("expected step status failed, got %q", resp.Steps[0].Status)
	}
	host := resp.Steps[0].Hosts["web"]
	if host.Status != "failed" {
		t.Fatalf("expected host status failed, got %q", host.Status)
	}
	if host.Message == "" {
		t.Fatalf("expected host message to be set")
	}
	if host.Output["stderr"] != "boom" {
		t.Fatalf("expected host stderr output, got %v", host.Output["stderr"])
	}
}
