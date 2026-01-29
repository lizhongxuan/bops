package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bops/internal/aiworkflow"
	"bops/internal/workflow"
)

func TestAIWorkflowNodeRegeneratePreservesTargets(t *testing.T) {
	srv := newTestServer(t)
	srv.aiClient = &stubAI{reply: `{"step":{"name":"updated","action":"cmd.run","with":{"cmd":"echo ok"}}}`}

	yamlText := `version: v0.1
name: demo
steps:
  - name: step1
    action: cmd.run
    targets: ["web"]
    with:
      cmd: echo hi
  - name: step2
    action: cmd.run
    targets: ["db"]
    with:
      cmd: echo there
`
	reqPayload := nodeRegenRequest{
		Node: aiworkflow.NodeSpec{
			ID:      "step-1",
			Index:   0,
			Name:    "step1",
			Action:  "cmd.run",
			With:    map[string]any{"cmd": "echo hi"},
			Targets: []string{"web"},
		},
		Neighbors: nodeRegenNeighbors{
			Next: []aiworkflow.NeighborSpec{{Name: "step2", Action: "cmd.run"}},
		},
		Workflow: nodeRegenWorkflow{YAML: yamlText},
		Intent:   "update step",
	}
	data, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ai/workflow/node-regenerate", bytes.NewReader(data))
	w := httptest.NewRecorder()

	srv.handleAIWorkflowNodeRegenerate(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	updated, err := workflow.Load([]byte(resp.YAML))
	if err != nil {
		t.Fatalf("load updated yaml: %v", err)
	}
	if len(updated.Steps) < 1 {
		t.Fatalf("expected steps")
	}
	if updated.Steps[0].Name != "updated" {
		t.Fatalf("expected updated name, got %q", updated.Steps[0].Name)
	}
	if len(updated.Steps[0].Targets) != 1 || updated.Steps[0].Targets[0] != "web" {
		t.Fatalf("expected targets preserved, got %+v", updated.Steps[0].Targets)
	}
}
