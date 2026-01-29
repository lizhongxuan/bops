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
	"bops/internal/nodetemplate"
	"bops/internal/workflowstore"
)

func newNodeTemplateTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	srv := &Server{
		mux:           http.NewServeMux(),
		store:         workflowstore.New(filepath.Join(dir, "workflows")),
		envStore:      envstore.New(filepath.Join(dir, "envs")),
		aiStore:       aistore.New(filepath.Join(dir, "ai_sessions")),
		nodeTemplates: nodetemplate.New(filepath.Join(dir, "node_templates")),
		aiPrompt:      "test",
	}
	srv.routes()
	return srv
}

func TestNodeTemplatesAPI(t *testing.T) {
	srv := newNodeTemplateTestServer(t)
	payload := bytes.NewBufferString(`{"name":"pkg_install","category":"actions","description":"install pkg","tags":["pkg"],"node":{"type":"action","name":"install pkg","data":{"action":"pkg.install","with":{"name":"nginx"}}}}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/node-templates/pkg_install", payload)
	putRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", putRec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/node-templates?search=pkg_install", nil)
	listRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(listRec, listReq)
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
		t.Fatalf("expected 1 template, got %d", len(listResp.Items))
	}
}
