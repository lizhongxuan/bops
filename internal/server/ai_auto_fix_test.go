package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bops/internal/aiworkflow"
)

func TestAIWorkflowAutoFixRun(t *testing.T) {
	srv := newTestServer(t)
	stub := &stubAI{reply: "ok"}
	pipeline, err := aiworkflow.New(aiworkflow.Config{
		Client:       stub,
		SystemPrompt: "test prompt",
		MaxRetries:   1,
	})
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
	srv.aiWorkflow = pipeline

	payload := `{"yaml":"version: v0.1\nname: demo\nsteps:\n  - name: ok\n    action: cmd.run\n    with:\n      cmd: echo hi\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/workflow/auto-fix-run", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handleAIWorkflowAutoFixRun(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: status") {
		t.Fatalf("expected status event, got body: %s", body)
	}
	if !strings.Contains(body, "event: result") {
		t.Fatalf("expected result event, got body: %s", body)
	}
	if !strings.Contains(body, "\"yaml\"") {
		t.Fatalf("expected yaml field, got body: %s", body)
	}
}
