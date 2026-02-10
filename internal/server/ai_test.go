package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"bops/internal/ai"
	"bops/internal/aistore"
	"bops/internal/aiworkflow"
	"bops/internal/envstore"
	"bops/internal/stepsstore"
)

type stubAI struct {
	reply string
	last  []ai.Message
}

func (s *stubAI) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	s.last = messages
	return s.reply, nil
}

type stubSequence struct {
	responses []string
	idx       int
	last      []ai.Message
}

func (s *stubSequence) Chat(_ context.Context, messages []ai.Message) (string, error) {
	if s.idx >= len(s.responses) {
		return "", errors.New("no response configured")
	}
	s.last = messages
	s.idx++
	return s.responses[s.idx-1], nil
}

type streamAI struct {
	intentJSON   string
	workflowJSON string
	thought      string
}

func (s *streamAI) Chat(_ context.Context, _ []ai.Message) (string, error) {
	return s.intentJSON, nil
}

func (s *streamAI) ChatStream(_ context.Context, _ []ai.Message, onDelta func(ai.StreamDelta)) (string, string, error) {
	if onDelta != nil {
		onDelta(ai.StreamDelta{Thought: s.thought})
		onDelta(ai.StreamDelta{Content: s.workflowJSON})
	}
	return s.workflowJSON, s.thought, nil
}

type streamSequence struct {
	responses []string
	thought   string
	idx       int
}

func (s *streamSequence) Chat(_ context.Context, _ []ai.Message) (string, error) {
	if s.idx >= len(s.responses) {
		return "", errors.New("no response configured")
	}
	reply := s.responses[s.idx]
	s.idx++
	return reply, nil
}

func (s *streamSequence) ChatStream(_ context.Context, _ []ai.Message, onDelta func(ai.StreamDelta)) (string, string, error) {
	if s.idx >= len(s.responses) {
		return "", "", errors.New("no response configured")
	}
	reply := s.responses[s.idx]
	s.idx++
	if onDelta != nil {
		onDelta(ai.StreamDelta{Thought: s.thought})
		onDelta(ai.StreamDelta{Content: reply})
	}
	return reply, s.thought, nil
}

type streamRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newStreamRecorder() *streamRecorder {
	return &streamRecorder{header: make(http.Header)}
}

func (r *streamRecorder) Header() http.Header {
	return r.header
}

func (r *streamRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *streamRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *streamRecorder) Flush() {}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	srv := &Server{
		mux:       http.NewServeMux(),
		store:     stepsstore.New(filepath.Join(dir, "workflows")),
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
	planJSON := `{"plan":[{"step_name":"install nginx","description":"install packages","dependencies":[]}],"missing":[]}`
	stepJSON := `{"tool":"step_patch","args":{"step_name":"install nginx","action":"cmd.run","targets":["local"],"args":{"cmd":"echo install nginx"},"summary":"install nginx"}}`
	stub := &stubSequence{responses: []string{planJSON, stepJSON}}
	srv.aiClient = stub
	workflow, err := aiworkflow.New(aiworkflow.Config{
		Client:       stub,
		SystemPrompt: srv.aiPrompt,
		MaxRetries:   2,
	})
	if err != nil {
		t.Fatalf("init ai workflow: %v", err)
	}
	srv.aiWorkflow = workflow

	body := bytes.NewBufferString(`{"prompt":"install nginx"}`)
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
	if !strings.Contains(stub.last[len(stub.last)-1].Content, "install nginx") {
		t.Fatalf("expected plan step context to be included in last message")
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

func TestBuildStreamMessageFromEvent(t *testing.T) {
	startEvt := aiworkflow.Event{
		Node:        "generator",
		Status:      "start",
		DisplayName: "生成工作流",
		Message:     "begin",
		AgentID:     "main",
		AgentName:   "main",
		AgentRole:   "primary",
	}
	msg, ok := buildStreamMessageFromEvent(startEvt, "reply-1", 1)
	if !ok {
		t.Fatalf("expected message to be built")
	}
	if msg.Type != "function_call" || msg.IsFinish {
		t.Fatalf("expected function_call not finished, got type=%s finish=%v", msg.Type, msg.IsFinish)
	}
	if msg.ExtraInfo.CallID != "generator" {
		t.Fatalf("expected call_id generator, got %s", msg.ExtraInfo.CallID)
	}
	if msg.ExtraInfo.ExecuteDisplayName == "" {
		t.Fatalf("expected execute_display_name to be set")
	}
	if msg.ExtraInfo.AgentID != "main" || msg.ExtraInfo.AgentName != "main" || msg.ExtraInfo.AgentRole != "primary" {
		t.Fatalf("expected agent fields to be set, got %+v", msg.ExtraInfo)
	}
	var names map[string]string
	if err := json.Unmarshal([]byte(msg.ExtraInfo.ExecuteDisplayName), &names); err != nil {
		t.Fatalf("decode execute_display_name: %v", err)
	}
	if names["name_executing"] == "" || names["name_executed"] == "" || names["name_execute_failed"] == "" {
		t.Fatalf("expected execute_display_name fields to be populated")
	}

	loopEvt := aiworkflow.Event{
		Node:        "tool",
		Status:      "start",
		LoopID:      "loop-1",
		Iteration:   2,
		AgentStatus: "tool_call",
	}
	loopMsg, ok := buildStreamMessageFromEvent(loopEvt, "reply-1", 6)
	if !ok {
		t.Fatalf("expected loop message to be built")
	}
	if loopMsg.ExtraInfo.LoopID != "loop-1" {
		t.Fatalf("expected loop_id loop-1, got %s", loopMsg.ExtraInfo.LoopID)
	}
	if loopMsg.ExtraInfo.Iteration != 2 {
		t.Fatalf("expected iteration 2, got %d", loopMsg.ExtraInfo.Iteration)
	}
	if loopMsg.ExtraInfo.AgentStatus != "tool_call" {
		t.Fatalf("expected agent_status tool_call, got %s", loopMsg.ExtraInfo.AgentStatus)
	}

	errorEvt := aiworkflow.Event{
		Node:   "validator",
		Status: "error",
	}
	errorMsg, ok := buildStreamMessageFromEvent(errorEvt, "reply-1", 2)
	if !ok {
		t.Fatalf("expected error message to be built")
	}
	if errorMsg.Type != "tool_response" || !errorMsg.IsFinish {
		t.Fatalf("expected tool_response finished, got type=%s finish=%v", errorMsg.Type, errorMsg.IsFinish)
	}
	if errorMsg.ExtraInfo.PluginStatus != "1" {
		t.Fatalf("expected plugin_status 1, got %s", errorMsg.ExtraInfo.PluginStatus)
	}

	streamEvt := aiworkflow.Event{
		Node:   "executor",
		Status: "stream",
		Data: map[string]any{
			"stream_plugin_running": "stream-123",
		},
	}
	streamMsg, ok := buildStreamMessageFromEvent(streamEvt, "reply-1", 3)
	if !ok {
		t.Fatalf("expected stream message to be built")
	}
	if streamMsg.Type != "tool_response" || streamMsg.IsFinish {
		t.Fatalf("expected stream tool_response not finished, got type=%s finish=%v", streamMsg.Type, streamMsg.IsFinish)
	}
	if streamMsg.ExtraInfo.StreamPluginRunning != "stream-123" {
		t.Fatalf("expected stream_plugin_running stream-123, got %s", streamMsg.ExtraInfo.StreamPluginRunning)
	}

	finishEvt := aiworkflow.Event{
		Node:   "executor",
		Status: "stream_finish",
		Data: map[string]any{
			"stream_plugin_running": "stream-123",
			"tool_output_content":   "final output",
		},
	}
	finishMsg, ok := buildStreamMessageFromEvent(finishEvt, "reply-1", 4)
	if !ok {
		t.Fatalf("expected stream finish message to be built")
	}
	if finishMsg.IsFinish != true {
		t.Fatalf("expected stream finish to be marked finished")
	}
	if finishMsg.Content != "final output" {
		t.Fatalf("expected tool_output_content to be used, got %s", finishMsg.Content)
	}

	verboseMsg, ok := buildStreamPluginFinishVerbose(finishEvt, "reply-1", 5)
	if !ok {
		t.Fatalf("expected stream plugin finish verbose message to be built")
	}
	if verboseMsg.Type != "verbose" {
		t.Fatalf("expected verbose message type, got %s", verboseMsg.Type)
	}
	if !strings.Contains(verboseMsg.Content, "stream_plugin_finish") {
		t.Fatalf("expected verbose content to include stream_plugin_finish")
	}
}

func TestAIWorkflowStreamSSEOrder(t *testing.T) {
	srv := newTestServer(t)
	planJSON := `{"plan":[{"step_name":"install nginx","description":"install packages","dependencies":[]}],"missing":[]}`
	stepJSON := `{"tool":"step_patch","args":{"step_name":"install nginx","action":"cmd.run","targets":["local"],"args":{"cmd":"echo install nginx"},"summary":"install nginx"}}`
	streamClient := &streamSequence{
		responses: []string{planJSON, stepJSON},
		thought:   "thinking...",
	}
	workflow, err := aiworkflow.New(aiworkflow.Config{
		Client:       streamClient,
		SystemPrompt: srv.aiPrompt,
		MaxRetries:   1,
	})
	if err != nil {
		t.Fatalf("init ai workflow: %v", err)
	}
	srv.aiWorkflow = workflow

	body := bytes.NewBufferString(`{"prompt":"install nginx"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/workflow/stream", body)
	rec := newStreamRecorder()

	srv.handleAIWorkflowStream(rec, req)
	if rec.status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.status)
	}
	output := rec.body.String()
	if !strings.Contains(output, "event: delta") {
		t.Fatalf("expected delta event in stream")
	}
	if !strings.Contains(output, "\"channel\":\"reasoning\"") {
		t.Fatalf("expected reasoning delta channel in stream")
	}
	if !strings.Contains(output, "event: status") {
		t.Fatalf("expected status event in stream")
	}
	if !strings.Contains(output, "event: message") {
		t.Fatalf("expected message event in stream")
	}
	if !strings.Contains(output, "\"type\":\"plan_ready\"") {
		t.Fatalf("expected plan_ready message in stream")
	}
	if !strings.Contains(output, "event: card") {
		t.Fatalf("expected card event in stream")
	}
	if !strings.Contains(output, "\"card_type\":\"plan_step\"") {
		t.Fatalf("expected plan_step card in stream")
	}
	if !strings.Contains(output, "event: result") {
		t.Fatalf("expected result event in stream")
	}
	if strings.Index(output, "event: delta") > strings.Index(output, "event: result") {
		t.Fatalf("expected delta to appear before result")
	}
	if strings.Index(output, "event: status") > strings.Index(output, "event: result") {
		t.Fatalf("expected status to appear before result")
	}
}
