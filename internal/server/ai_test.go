package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"bops/internal/ai"
	"bops/internal/aistore"
	"bops/internal/envstore"
	"bops/internal/workflowstore"
)

type stubAI struct {
	reply string
	last  []ai.Message
}

func (s *stubAI) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	s.last = messages
	return s.reply, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	srv := &Server{
		mux:       http.NewServeMux(),
		store:     workflowstore.New(filepath.Join(dir, "workflows")),
		envStore:  envstore.New(filepath.Join(dir, "envs")),
		aiStore:   aistore.New(filepath.Join(dir, "ai_sessions")),
		aiPrompt:  "test prompt",
		bus:       nil,
		engine:    nil,
		runs:      nil,
		StaticDir: "",
	}
	srv.routes()
	return srv
}

func TestAIWorkflowGenerate(t *testing.T) {
	srv := newTestServer(t)
	stub := &stubAI{reply: "```yaml\nversion: v0.1\nname: demo\nsteps:\n  - name: ok\n    action: cmd.run\n    with:\n      cmd: \"echo hi\"\n```"}
	srv.aiClient = stub

	body := bytes.NewBufferString(`{"prompt":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/workflow/generate", body)
	w := httptest.NewRecorder()

	srv.handleAIWorkflowGenerate(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.YAML == "" {
		t.Fatalf("expected yaml in response")
	}
	if stub.last == nil || len(stub.last) < 2 {
		t.Fatalf("expected messages to be sent")
	}
	if stub.last[0].Role != "system" {
		t.Fatalf("expected system prompt to be included")
	}
	if stub.last[len(stub.last)-1].Content != "hello" {
		t.Fatalf("expected user prompt to be last message")
	}
}

func TestAIChatSessionFlow(t *testing.T) {
	srv := newTestServer(t)
	stub := &stubAI{reply: "hi there"}
	srv.aiClient = stub

	createReq := httptest.NewRequest(http.MethodPost, "/api/ai/chat/sessions", bytes.NewBufferString(`{}`))
	createRec := httptest.NewRecorder()
	srv.handleAIChatSessions(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", createRec.Code)
	}

	var created struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Session.ID == "" {
		t.Fatalf("expected session id")
	}

	messageReq := httptest.NewRequest(http.MethodPost, "/api/ai/chat/sessions/"+created.Session.ID+"/messages", bytes.NewBufferString(`{"content":"ping"}`))
	messageRec := httptest.NewRecorder()
	srv.handleAIChatSession(messageRec, messageReq)
	if messageRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", messageRec.Code)
	}

	var msgResp struct {
		Session struct {
			Messages []ai.Message `json:"messages"`
		} `json:"session"`
		Reply *ai.Message `json:"reply"`
	}
	if err := json.NewDecoder(messageRec.Body).Decode(&msgResp); err != nil {
		t.Fatalf("decode message response: %v", err)
	}
	if len(msgResp.Session.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgResp.Session.Messages))
	}
	if msgResp.Reply == nil || msgResp.Reply.Content != "hi there" {
		replyText := ""
		if msgResp.Reply != nil {
			replyText = msgResp.Reply.Content
		}
		t.Fatalf("unexpected reply: %s", replyText)
	}
}
