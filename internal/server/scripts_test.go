package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"bops/internal/aistore"
	"bops/internal/envstore"
	"bops/internal/scriptstore"
	"bops/internal/workflowstore"
)

func newScriptTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	srv := &Server{
		mux:         http.NewServeMux(),
		store:       workflowstore.New(filepath.Join(dir, "workflows")),
		envStore:    envstore.New(filepath.Join(dir, "envs")),
		aiStore:     aistore.New(filepath.Join(dir, "ai_sessions")),
		scriptStore: scriptstore.New(filepath.Join(dir, "scripts")),
		aiPrompt:    "test",
	}
	srv.routes()
	return srv
}

func TestScriptsAPI(t *testing.T) {
	srv := newScriptTestServer(t)

	payload := bytes.NewBufferString(`{"name":"demo","language":"shell","description":"test script","tags":["ops"],"content":"echo hi"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/scripts/demo", payload)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/scripts?search=ops", nil)
	listRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(listRec, reqList)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", listRec.Code)
	}

	var listResp struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Items) != 1 {
		t.Fatalf("expected 1 script, got %d", len(listResp.Items))
	}
}
