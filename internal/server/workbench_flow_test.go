package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bops/internal/ai"
	"bops/internal/aistore"
	"bops/internal/aiworkflow"
	"bops/internal/aiworkflowstore"
	"bops/internal/engine"
	"bops/internal/envstore"
	"bops/internal/eventbus"
	"bops/internal/modules"
	"bops/internal/runmanager"
	"bops/internal/state"
	"bops/internal/workflowstore"
)

type flowAI struct{}

func (f *flowAI) Chat(_ context.Context, messages []ai.Message) (string, error) {
	prompt := ""
	if len(messages) > 0 {
		prompt = messages[len(messages)-1].Content
	}
	if strings.Contains(prompt, "step must include") {
		return `{"step":{"name":"updated-step","action":"cmd.run","with":{"cmd":"echo updated"}},"questions":[]}`, nil
	}
	return `{"workflow":{"version":"v0.1","name":"demo","description":"demo","inventory":{"hosts":{"web":{"address":"127.0.0.1"}}},"plan":{"mode":"auto","strategy":"sequential"},"steps":[{"name":"install","action":"cmd.run","with":{"cmd":"echo hi"}}]},"questions":[]}`, nil
}

type noopModule struct {
	delay time.Duration
}

func (m noopModule) Check(ctx context.Context, req modules.Request) (modules.Result, error) {
	return modules.Result{Changed: true}, nil
}

func (m noopModule) Apply(ctx context.Context, req modules.Request) (modules.Result, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return modules.Result{Output: map[string]any{"ok": true}}, nil
}

func (m noopModule) Rollback(ctx context.Context, req modules.Request) (modules.Result, error) {
	return modules.Result{}, nil
}

func newWorkbenchFlowServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	bus := eventbus.New()
	runs := runmanager.NewWithBus(state.NewFileStore(filepath.Join(dir, "state.json")), bus)
	registry := modules.NewRegistry()
	if err := registry.Register("cmd.run", noopModule{delay: 80 * time.Millisecond}); err != nil {
		t.Fatalf("register module: %v", err)
	}
	eng := engine.New(registry)
	aiClient := &flowAI{}
	pipeline, err := aiworkflow.New(aiworkflow.Config{
		Client:       aiClient,
		SystemPrompt: "test prompt",
		MaxRetries:   1,
	})
	if err != nil {
		t.Fatalf("new ai workflow: %v", err)
	}
	srv := &Server{
		mux:             http.NewServeMux(),
		store:           workflowstore.New(filepath.Join(dir, "workflows")),
		envStore:        envstore.New(filepath.Join(dir, "envs")),
		aiStore:         aistore.New(filepath.Join(dir, "ai_sessions")),
		aiWorkflowStore: aiworkflowstore.New(filepath.Join(dir, "ai_workflow_drafts")),
		aiClient:        aiClient,
		aiWorkflow:      pipeline,
		aiPrompt:        "test",
		runs:            runs,
		bus:             bus,
		engine:          eng,
	}
	srv.routes()
	return srv
}

func TestWorkbenchCoreFlow(t *testing.T) {
	srv := newWorkbenchFlowServer(t)
	server := httptest.NewServer(srv.mux)
	defer server.Close()
	client := server.Client()

	genPayload := bytes.NewBufferString(`{"prompt":"install nginx"}`)
	genResp, err := client.Post(server.URL+"/api/ai/workflow/generate", "application/json", genPayload)
	if err != nil {
		t.Fatalf("generate request: %v", err)
	}
	defer genResp.Body.Close()
	if genResp.StatusCode != http.StatusOK {
		t.Fatalf("generate status %d", genResp.StatusCode)
	}
	var gen struct {
		YAML    string `json:"yaml"`
		DraftID string `json:"draft_id"`
	}
	if err := json.NewDecoder(genResp.Body).Decode(&gen); err != nil {
		t.Fatalf("decode generate: %v", err)
	}
	if strings.TrimSpace(gen.YAML) == "" {
		t.Fatalf("expected yaml from generate")
	}
	if strings.TrimSpace(gen.DraftID) == "" {
		t.Fatalf("expected draft id from generate")
	}

	graphReq := bytes.NewBufferString(`{"yaml":` + toJSON(gen.YAML) + `}`)
	graphResp, err := client.Post(server.URL+"/api/ai/workflow/graph-from-yaml", "application/json", graphReq)
	if err != nil {
		t.Fatalf("graph request: %v", err)
	}
	defer graphResp.Body.Close()
	if graphResp.StatusCode != http.StatusOK {
		t.Fatalf("graph status %d", graphResp.StatusCode)
	}
	var graph struct {
		Graph struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"graph"`
	}
	if err := json.NewDecoder(graphResp.Body).Decode(&graph); err != nil {
		t.Fatalf("decode graph: %v", err)
	}
	if len(graph.Graph.Nodes) == 0 {
		t.Fatalf("expected graph nodes")
	}

	draftBody := map[string]any{
		"id":    gen.DraftID,
		"yaml":  gen.YAML,
		"graph": graph.Graph,
	}
	draftData, _ := json.Marshal(draftBody)
	draftReq, _ := http.NewRequest(http.MethodPut, server.URL+"/api/ai/workflow/draft/"+gen.DraftID, bytes.NewReader(draftData))
	draftReq.Header.Set("Content-Type", "application/json")
	draftResp, err := client.Do(draftReq)
	if err != nil {
		t.Fatalf("draft save: %v", err)
	}
	defer draftResp.Body.Close()
	if draftResp.StatusCode != http.StatusOK {
		t.Fatalf("draft status %d", draftResp.StatusCode)
	}

	regenBody := map[string]any{
		"node": map[string]any{
			"id":     "step-1",
			"index":  0,
			"name":   graph.Graph.Nodes[0].Name,
			"action": "cmd.run",
			"with":   map[string]any{"cmd": "echo hi"},
		},
		"neighbors": map[string]any{
			"prev": []any{},
			"next": []any{},
		},
		"workflow": map[string]any{"yaml": gen.YAML},
		"intent":   "update step",
	}
	regenData, _ := json.Marshal(regenBody)
	regenResp, err := client.Post(server.URL+"/api/ai/workflow/node-regenerate", "application/json", bytes.NewReader(regenData))
	if err != nil {
		t.Fatalf("regen request: %v", err)
	}
	defer regenResp.Body.Close()
	if regenResp.StatusCode != http.StatusOK {
		t.Fatalf("regen status %d", regenResp.StatusCode)
	}
	var regen struct {
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(regenResp.Body).Decode(&regen); err != nil {
		t.Fatalf("decode regen: %v", err)
	}
	if !strings.Contains(regen.YAML, "updated-step") {
		t.Fatalf("expected updated step in yaml")
	}

	autoFixPayload := bytes.NewBufferString(`{"yaml":` + toJSON(regen.YAML) + `}`)
	autoFixResp, err := client.Post(server.URL+"/api/ai/workflow/auto-fix-run", "application/json", autoFixPayload)
	if err != nil {
		t.Fatalf("auto-fix request: %v", err)
	}
	defer autoFixResp.Body.Close()
	if autoFixResp.StatusCode != http.StatusOK {
		t.Fatalf("auto-fix status %d", autoFixResp.StatusCode)
	}
	autoFixBody := readAll(autoFixResp.Body)
	if !strings.Contains(autoFixBody, "event: result") {
		t.Fatalf("expected auto-fix result event")
	}

	runPayload := bytes.NewBufferString(`{"yaml":` + toJSON(regen.YAML) + `}`)
	runResp, err := client.Post(server.URL+"/api/runs/workflow", "application/json", runPayload)
	if err != nil {
		t.Fatalf("run request: %v", err)
	}
	defer runResp.Body.Close()
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("run status %d", runResp.StatusCode)
	}
	var run struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(runResp.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.RunID == "" {
		t.Fatalf("expected run id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	streamReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/runs/"+run.RunID+"/stream", nil)
	streamResp, err := client.Do(streamReq)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer streamResp.Body.Close()

	found := false
	err = readSSE(streamResp.Body, func(name, data string) bool {
		if name != "workflow_end" {
			return false
		}
		found = true
		var payload struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal([]byte(data), &payload); err == nil {
			if payload.Data["status"] != "success" {
				t.Fatalf("expected success status, got %v", payload.Data["status"])
			}
			if payload.Data["total_steps"] == nil {
				t.Fatalf("expected total_steps in summary")
			}
		}
		return true
	})
	if err != nil && ctx.Err() == nil {
		t.Fatalf("stream read: %v", err)
	}
	if !found {
		t.Fatalf("expected workflow_end event")
	}
}

func toJSON(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func readAll(body io.Reader) string {
	data, _ := io.ReadAll(body)
	return string(data)
}

func readSSE(body io.Reader, onEvent func(name, data string) bool) error {
	scanner := bufio.NewScanner(body)
	var (
		eventName string
		data      string
	)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if eventName != "" || data != "" {
				if onEvent(eventName, data) {
					return nil
				}
			}
			eventName = ""
			data = ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	return scanner.Err()
}
