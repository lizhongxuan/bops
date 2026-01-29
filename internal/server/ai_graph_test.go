package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIWorkflowGraphFromYAML(t *testing.T) {
	srv := newTestServer(t)
	bodyPayload := struct {
		YAML string `json:"yaml"`
	}{
		YAML: "version: v0.1\nname: demo\nsteps:\n  - name: step1\n    action: cmd.run\n    with:\n      cmd: \"echo hi\"\n",
	}
	data, err := json.Marshal(bodyPayload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ai/workflow/graph-from-yaml", bytes.NewReader(data))
	w := httptest.NewRecorder()

	srv.handleAIWorkflowGraphFromYAML(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Graph struct {
			Version string `json:"version"`
			Nodes   []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
			Edges []struct {
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"edges"`
		} `json:"graph"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Graph.Version == "" {
		t.Fatalf("expected graph version")
	}
	if len(resp.Graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(resp.Graph.Nodes))
	}
	if resp.Graph.Nodes[0].Name != "step1" {
		t.Fatalf("expected node name step1, got %q", resp.Graph.Nodes[0].Name)
	}
	if len(resp.Graph.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(resp.Graph.Edges))
	}
}
