package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"bops/internal/aistore"
	"bops/internal/envstore"
	"bops/internal/stepsstore"
	"bops/runner/engine"
	"bops/runner/scriptstore"
)

func newPartsTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	scripts := scriptstore.New(filepath.Join(dir, "scripts"))
	return &Server{
		mux:         http.NewServeMux(),
		store:       stepsstore.New(filepath.Join(dir, "workflows")),
		envStore:    envstore.New(filepath.Join(dir, "envs")),
		aiStore:     aistore.New(filepath.Join(dir, "ai_sessions")),
		scriptStore: scripts,
		engine:      engine.New(defaultRegistry(scripts)),
		aiPrompt:    "test",
	}
}

func TestInventoryAccessRestricted(t *testing.T) {
	srv := newPartsTestServer(t)
	srv.routes()

	stepsYAML := []byte(`version: v0.1
name: demo
steps:
  - name: step1
    action: cmd.run
    args:
      cmd: "echo hi"
`)
	if _, err := srv.store.PutSteps("demo", stepsYAML); err != nil {
		t.Fatalf("put steps: %v", err)
	}
	invYAML := []byte(`inventory:
  hosts:
    local:
      address: "127.0.0.1"
`)
	if _, err := srv.store.PutInventory("demo", invYAML); err != nil {
		t.Fatalf("put inventory: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/workflows/demo/inventory", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for inventory without header, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/workflows/demo/inventory", nil)
	req.Header.Set("X-Workflow-Editor", "manual")
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for inventory with header, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/workflows/demo/steps", nil)
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for steps, got %d", rec.Code)
	}
}

func TestPlanWithSplitWorkflow(t *testing.T) {
	srv := newPartsTestServer(t)
	srv.routes()

	stepsYAML := []byte(`version: v0.1
name: demo
steps:
  - name: step1
    action: cmd.run
    args:
      cmd: "echo hi"
`)
	if _, err := srv.store.PutSteps("demo", stepsYAML); err != nil {
		t.Fatalf("put steps: %v", err)
	}
	invYAML := []byte(`inventory:
  hosts:
    local:
      address: "127.0.0.1"
`)
	if _, err := srv.store.PutInventory("demo", invYAML); err != nil {
		t.Fatalf("put inventory: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/workflows/demo/plan", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from plan, got %d", rec.Code)
	}
}
