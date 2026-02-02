package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"bops/internal/ai"
	"bops/internal/aistore"
	"bops/internal/aiworkflow"
	"bops/internal/aiworkflowstore"
	"bops/internal/logging"
	"bops/internal/skills"
	"bops/internal/validationenv"
	"bops/internal/validationrun"
	"bops/internal/workflow"
	"github.com/cloudwego/eino/components/tool"
	"go.uber.org/zap"
)

const maxChatContextMessages = 20

type aiSessionCreateRequest struct {
	Title string `json:"title"`
}

type aiSessionListResponse struct {
	Items []aistore.Summary `json:"items"`
	Total int               `json:"total"`
}

type aiSessionResponse struct {
	Session aistore.Session `json:"session"`
}

type aiChatRequest struct {
	Content string `json:"content"`
	Role    string `json:"role,omitempty"`
	SkipAI  bool   `json:"skip_ai,omitempty"`
}

type aiChatResponse struct {
	Reply   *ai.Message     `json:"reply,omitempty"`
	Session aistore.Session `json:"session"`
}

type aiGenerateRequest struct {
	Prompt  string         `json:"prompt"`
	Context map[string]any `json:"context,omitempty"`
	DraftID string         `json:"draft_id,omitempty"`
	YAML    string         `json:"yaml,omitempty"`
}

type aiGenerateResponse struct {
	YAML    string `json:"yaml"`
	Message string `json:"message,omitempty"`
	DraftID string `json:"draft_id,omitempty"`
}

type aiFixRequest struct {
	YAML    string   `json:"yaml"`
	Issues  []string `json:"issues,omitempty"`
	DraftID string   `json:"draft_id,omitempty"`
}

type aiValidateRequest struct {
	YAML string `json:"yaml"`
}

type aiValidateResponse struct {
	OK     bool     `json:"ok"`
	Issues []string `json:"issues,omitempty"`
}

type aiExecuteRequest struct {
	YAML string `json:"yaml"`
	Env  string `json:"env,omitempty"`
}

type aiExecuteResponse struct {
	Status string `json:"status"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	Code   int    `json:"code,omitempty"`
	Error  string `json:"error,omitempty"`
}

type aiSummaryRequest struct {
	YAML string `json:"yaml"`
}

type aiSummaryResponse struct {
	Summary     string   `json:"summary"`
	Steps       int      `json:"steps"`
	RiskLevel   string   `json:"risk_level"`
	RiskNotes   []string `json:"risk_notes,omitempty"`
	Issues      []string `json:"issues,omitempty"`
	NeedsReview bool     `json:"needs_review"`
}

type aiStreamRequest struct {
	Mode         string         `json:"mode,omitempty"`
	AgentMode    string         `json:"agent_mode,omitempty"`
	AgentName    string         `json:"agent_name,omitempty"`
	Agents       []string       `json:"agents,omitempty"`
	SessionKey   string         `json:"session_key,omitempty"`
	Prompt       string         `json:"prompt,omitempty"`
	Script       string         `json:"script,omitempty"`
	YAML         string         `json:"yaml,omitempty"`
	Issues       []string       `json:"issues,omitempty"`
	Context      map[string]any `json:"context,omitempty"`
	Env          string         `json:"env,omitempty"`
	Execute      bool           `json:"execute,omitempty"`
	MaxRetries   int            `json:"max_retries,omitempty"`
	LoopMaxIters int            `json:"loop_max_iters,omitempty"`
	DraftID      string         `json:"draft_id,omitempty"`
}

const (
	cardTypeFileCreate = "file_create"
)

type streamMessage struct {
	MessageID        string           `json:"message_id"`
	ReplyID          string           `json:"reply_id"`
	Role             string           `json:"role"`
	Type             string           `json:"type"`
	Content          string           `json:"content"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	IsFinish         bool             `json:"is_finish"`
	Index            int              `json:"index"`
	ExtraInfo        streamMessageExt `json:"extra_info"`
}

type streamMessageExt struct {
	CallID              string `json:"call_id,omitempty"`
	ExecuteDisplayName  string `json:"execute_display_name,omitempty"`
	PluginStatus        string `json:"plugin_status,omitempty"`
	MessageTitle        string `json:"message_title,omitempty"`
	StreamPluginRunning string `json:"stream_plugin_running,omitempty"`
	LoopID              string `json:"loop_id,omitempty"`
	Iteration           int    `json:"iteration,omitempty"`
	AgentStatus         string `json:"agent_status,omitempty"`
	AgentID             string `json:"agent_id,omitempty"`
	AgentName           string `json:"agent_name,omitempty"`
	AgentRole           string `json:"agent_role,omitempty"`
}

func (s *Server) handleAIChatSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.aiStore.List()
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, aiSessionListResponse{Items: items, Total: len(items)})
	case http.MethodPost:
		body, err := readBody(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		var req aiSessionCreateRequest
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid json payload")
				return
			}
		}
		session, err := s.aiStore.Create(req.Title)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, aiSessionResponse{Session: session})
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAIChatSession(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/ai/chat/sessions/")
	if strings.HasSuffix(path, "/messages") {
		id := strings.TrimSuffix(path, "/messages")
		s.handleAIChatMessages(w, r, id)
		return
	}

	id := strings.Trim(path, "/")
	if id == "" {
		writeError(w, r, http.StatusNotFound, "session id is required")
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	session, _, err := s.aiStore.Get(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, aiSessionResponse{Session: session})
}

func (s *Server) handleAIChatMessages(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id = strings.Trim(id, "/")
	if id == "" {
		writeError(w, r, http.StatusNotFound, "session id is required")
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req aiChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, r, http.StatusBadRequest, "content is required")
		return
	}

	session, _, err := s.aiStore.Get(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, err.Error())
		return
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "assistant" && role != "system" {
		writeError(w, r, http.StatusBadRequest, "invalid role")
		return
	}

	if req.SkipAI || s.aiClient == nil {
		msg := ai.Message{Role: role, Content: strings.TrimSpace(req.Content)}
		session.Messages = append(session.Messages, msg)
		if session.Title == "新会话" && role == "user" && strings.TrimSpace(session.Messages[0].Content) != "" {
			session.Title = titleFromMessage(session.Messages[0].Content)
		}
		if err := s.aiStore.Save(session); err != nil {
			writeError(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, aiChatResponse{Session: session})
		return
	}

	if s.aiClient == nil {
		writeError(w, r, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}

	userMsg := ai.Message{Role: role, Content: strings.TrimSpace(req.Content)}
	messages := s.buildChatMessages(session.Messages, userMsg)

	reply, err := s.aiClient.Chat(r.Context(), messages)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	assistantMsg := ai.Message{Role: "assistant", Content: strings.TrimSpace(reply)}
	session.Messages = append(session.Messages, userMsg, assistantMsg)
	if session.Title == "新会话" && strings.TrimSpace(session.Messages[0].Content) != "" {
		session.Title = titleFromMessage(session.Messages[0].Content)
	}
	if err := s.aiStore.Save(session); err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, aiChatResponse{Reply: &assistantMsg, Session: session})
}

func (s *Server) handleAIWorkflowGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.aiWorkflow == nil {
		writeError(w, r, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req aiGenerateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeError(w, r, http.StatusBadRequest, "prompt is required")
		return
	}

	contextText := s.buildContextText(req.Context)
	baseYAML := strings.TrimSpace(req.YAML)
	if baseYAML != "" && countStepsInYAML(baseYAML) == 0 {
		baseYAML = ""
	}
	state, err := s.aiWorkflow.RunGenerate(r.Context(), strings.TrimSpace(req.Prompt), req.Context, aiworkflow.RunOptions{
		SystemPrompt:  s.systemPrompt(contextText),
		ContextText:   contextText,
		ValidationEnv: s.defaultValidationEnv(),
		SkipExecute:   true,
		BaseYAML:      baseYAML,
	})
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	draftID := s.saveAIDraft(req.DraftID, titleFromMessage(req.Prompt), req.Prompt, state)
	writeJSON(w, http.StatusOK, aiGenerateResponse{
		YAML:    state.YAML,
		Message: state.Summary,
		DraftID: draftID,
	})
}

func (s *Server) handleAIWorkflowFix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.aiWorkflow == nil {
		writeError(w, r, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req aiFixRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}

	baseYAML := strings.TrimSpace(req.YAML)
	state, err := s.aiWorkflow.RunFix(r.Context(), req.YAML, req.Issues, aiworkflow.RunOptions{
		SystemPrompt:  s.systemPrompt(""),
		ValidationEnv: s.defaultValidationEnv(),
		SkipExecute:   true,
		BaseYAML:      baseYAML,
	})
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	title := ""
	if strings.TrimSpace(req.DraftID) == "" {
		title = "AI Fix"
	}
	draftID := s.saveAIDraft(req.DraftID, title, "", state)
	writeJSON(w, http.StatusOK, aiGenerateResponse{
		YAML:    state.YAML,
		Message: state.Summary,
		DraftID: draftID,
	})
}

func (s *Server) handleAIWorkflowValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req aiValidateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}

	resp := aiValidateResponse{OK: true}
	wf, err := workflow.Load([]byte(req.YAML))
	if err != nil {
		resp.OK = false
		resp.Issues = []string{err.Error()}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if err := wf.Validate(); err != nil {
		resp.OK = false
		if vErr, ok := err.(*workflow.ValidationError); ok {
			resp.Issues = vErr.Issues
		} else {
			resp.Issues = []string{err.Error()}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAIWorkflowExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req aiExecuteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}

	var env *validationenv.ValidationEnv
	if strings.TrimSpace(req.Env) != "" && s.validationStore != nil {
		if resolved, _, err := s.validationStore.Get(req.Env); err == nil {
			env = &resolved
		}
	}
	if env == nil {
		env = s.defaultValidationEnv()
	}
	if env == nil {
		writeError(w, r, http.StatusBadRequest, "validation env is required")
		return
	}

	result, runErr := validationrun.Runner(r.Context(), *env, req.YAML)
	resp := aiExecuteResponse{
		Status: result.Status,
		Stdout: result.Stdout,
		Stderr: result.Stderr,
		Code:   result.Code,
	}
	if runErr != nil {
		resp.Status = "failed"
		resp.Error = runErr.Error()
	}
	s.recordValidationAudit(validationAuditEntry{
		Source:    "ai-workflow",
		Env:       env.Name,
		EnvType:   string(env.Type),
		Status:    resp.Status,
		Code:      resp.Code,
		Error:     resp.Error,
		YAMLHash:  hashYAML(req.YAML),
		StepCount: countStepsInYAML(req.YAML),
	})
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAIWorkflowSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req aiSummaryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}

	riskLevel, riskNotes := aiworkflow.EvaluateRisk(req.YAML, aiworkflow.DefaultRiskRules())
	issues := []string{}
	ok := true
	if wf, err := workflow.Load([]byte(req.YAML)); err != nil {
		ok = false
		issues = []string{err.Error()}
	} else if err := wf.Validate(); err != nil {
		ok = false
		if vErr, okCast := err.(*workflow.ValidationError); okCast {
			issues = vErr.Issues
		} else {
			issues = []string{err.Error()}
		}
	}

	steps := countStepsInYAML(req.YAML)
	needsReview := !ok || riskLevel != aiworkflow.RiskLevelLow
	summary := fmt.Sprintf("steps=%d risk=%s issues=%d", steps, riskLevel, len(issues))

	writeJSON(w, http.StatusOK, aiSummaryResponse{
		Summary:     summary,
		Steps:       steps,
		RiskLevel:   string(riskLevel),
		RiskNotes:   riskNotes,
		Issues:      issues,
		NeedsReview: needsReview,
	})
}

func (s *Server) handleAIWorkflowStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.aiWorkflow == nil {
		writeError(w, r, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "streaming unsupported")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req aiStreamRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = string(aiworkflow.ModeGenerate)
	}
	if strings.TrimSpace(req.AgentName) == "" && len(req.Agents) == 0 {
		if strings.TrimSpace(s.cfg.DefaultAgent) != "" {
			req.AgentName = strings.TrimSpace(s.cfg.DefaultAgent)
		}
		if len(s.cfg.DefaultAgents) > 0 {
			req.Agents = append([]string{}, s.cfg.DefaultAgents...)
		}
	}
	agentMode := strings.ToLower(strings.TrimSpace(req.AgentMode))
	if agentMode == "" {
		if len(req.Agents) > 0 {
			agentMode = "multi"
		} else if mode == string(aiworkflow.ModeGenerate) {
			agentMode = "loop"
		} else {
			agentMode = "pipeline"
		}
	}
	if agentMode != "loop" && agentMode != "multi" {
		agentMode = "pipeline"
	}
	if mode == string(aiworkflow.ModeFix) {
		agentMode = "pipeline"
	}
	if mode == string(aiworkflow.ModeGenerate) && strings.TrimSpace(req.Prompt) == "" {
		writeError(w, r, http.StatusBadRequest, "prompt is required")
		return
	}
	if mode == string(aiworkflow.ModeFix) && strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}
	if mode == "simulate" && strings.TrimSpace(req.YAML) == "" {
		writeError(w, r, http.StatusBadRequest, "yaml is required")
		return
	}
	if mode == "migrate" && strings.TrimSpace(firstNonEmpty(req.Script, req.Prompt)) == "" {
		writeError(w, r, http.StatusBadRequest, "script is required")
		return
	}

	var env *validationenv.ValidationEnv
	if strings.TrimSpace(req.Env) != "" && s.validationStore != nil {
		if resolved, _, err := s.validationStore.Get(req.Env); err == nil {
			env = &resolved
		}
	}
	if env == nil {
		env = s.defaultValidationEnv()
	}
	envName := ""
	envType := ""
	if env != nil {
		envName = env.Name
		envType = string(env.Type)
	}
	replyID := fmt.Sprintf("reply-%d", time.Now().UnixNano())
	eventIndex := 0
	logging.L().Debug("ai workflow stream start",
		zap.String("mode", mode),
		zap.String("agent_mode", agentMode),
		zap.Int("prompt_len", len(strings.TrimSpace(req.Prompt))),
		zap.Int("yaml_len", len(req.YAML)),
		zap.Int("issues", len(req.Issues)),
		zap.Bool("execute", req.Execute),
		zap.Int("max_retries", req.MaxRetries),
		zap.String("env", envName),
		zap.String("draft_id", strings.TrimSpace(req.DraftID)),
	)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	events := make(chan aiworkflow.Event, 16)
	streamCh := make(chan ai.StreamDelta, 32)
	type streamResult struct {
		state      *aiworkflow.State
		err        error
		simulation *aiworkflow.SimulationResult
		migration  string
	}
	resultCh := make(chan streamResult, 1)

	sink := func(evt aiworkflow.Event) {
		select {
		case events <- evt:
		case <-ctx.Done():
		}
	}
	streamSink := func(delta ai.StreamDelta) {
		select {
		case streamCh <- delta:
		case <-ctx.Done():
		}
	}

	contextText := s.buildContextText(req.Context)
	baseYAML := strings.TrimSpace(req.YAML)
	if baseYAML != "" && countStepsInYAML(baseYAML) == 0 {
		baseYAML = ""
	}
	systemPrompt := s.systemPrompt(contextText)
	opts := aiworkflow.RunOptions{
		SystemPrompt:  systemPrompt,
		ContextText:   contextText,
		ValidationEnv: env,
		SkipExecute:   !req.Execute,
		MaxRetries:    req.MaxRetries,
		EventSink:     sink,
		StreamSink:    streamSink,
		BaseYAML:      baseYAML,
	}
	if sessionKey := strings.TrimSpace(req.SessionKey); sessionKey != "" {
		opts.SessionKey = sessionKey
	} else if strings.TrimSpace(req.DraftID) != "" {
		opts.SessionKey = strings.TrimSpace(req.DraftID)
	}
	if strings.TrimSpace(req.AgentName) != "" {
		opts.AgentSpec = aiworkflow.AgentSpec{Name: strings.TrimSpace(req.AgentName)}
	}
	if agentMode == "loop" {
		toolExecutor, toolNames := s.buildLoopToolExecutor(opts.AgentSpec)
		opts.SystemPrompt = s.loopSystemPrompt(contextText)
		opts.ToolExecutor = toolExecutor
		opts.ToolNames = toolNames
		opts.LoopMaxIters = req.LoopMaxIters
		opts.FallbackToPipeline = true
		opts.FallbackSystemPrompt = systemPrompt
	}

	go func() {
		defer close(events)
		defer close(streamCh)
		var state *aiworkflow.State
		var runErr error
		if mode == "simulate" {
			simResult, simErr := aiworkflow.RunSimulation(req.YAML, req.Context)
			summary := ""
			if simResult != nil {
				summary = simResult.Summary
			}
			state = &aiworkflow.State{
				Mode:    aiworkflow.ModeGenerate,
				Prompt:  strings.TrimSpace(req.Prompt),
				Context: req.Context,
				YAML:    strings.TrimSpace(req.YAML),
				Summary: summary,
			}
			resultCh <- streamResult{state: state, err: simErr, simulation: simResult}
			return
		}
		if mode == "migrate" {
			scriptText := firstNonEmpty(req.Script, req.Prompt)
			migrated, migErr := aiworkflow.ConvertScriptToYAML(scriptText)
			state = &aiworkflow.State{
				Mode:    aiworkflow.ModeGenerate,
				Prompt:  strings.TrimSpace(scriptText),
				Context: req.Context,
				YAML:    migrated,
				Summary: "migration completed",
			}
			resultCh <- streamResult{state: state, err: migErr, migration: migrated}
			return
		}
		if mode == string(aiworkflow.ModeFix) {
			state, runErr = s.aiWorkflow.RunFix(ctx, req.YAML, req.Issues, opts)
		} else if agentMode == "loop" {
			state, runErr = s.aiWorkflow.RunAgentLoop(ctx, strings.TrimSpace(req.Prompt), req.Context, opts)
		} else if agentMode == "multi" {
			specs := buildAgentSpecs(req.AgentName, req.Agents)
			state, runErr = s.aiWorkflow.RunMultiAgent(ctx, strings.TrimSpace(req.Prompt), req.Context, specs, opts)
		} else if strings.TrimSpace(req.AgentName) != "" {
			state, runErr = s.aiWorkflow.RunAgent(ctx, strings.TrimSpace(req.Prompt), req.Context, opts)
		} else {
			state, runErr = s.aiWorkflow.RunGenerate(ctx, strings.TrimSpace(req.Prompt), req.Context, opts)
		}
		resultCh <- streamResult{state: state, err: runErr}
	}()

	var pending *streamResult
	sentReasoningDelta := false

	for {
		select {
		case evt, ok := <-events:
			if ok {
				writeSSE(w, "status", evt)
				if evt.Status == "stream_finish" || evt.Status == "stream_done" {
					streamUUID := streamPluginRunningValue(evt)
					if streamUUID == "" {
						if msg, ok := buildStreamMessageFromEvent(evt, replyID, eventIndex); ok {
							writeSSE(w, "message", msg)
							eventIndex++
						}
					}
					if verboseMsg, ok := buildStreamPluginFinishVerbose(evt, replyID, eventIndex); ok {
						writeSSE(w, "message", verboseMsg)
						eventIndex++
					}
				} else {
					if msg, ok := buildStreamMessageFromEvent(evt, replyID, eventIndex); ok {
						writeSSE(w, "message", msg)
						eventIndex++
					}
				}
				if evt.Node == "generator" && evt.Status == "done" {
					if yamlText, ok := evt.Data["yaml"].(string); ok {
						trimmedYAML := strings.TrimSpace(yamlText)
						if trimmedYAML != "" {
							writeSSE(w, "card", map[string]any{
								"card_id":   "file-create",
								"reply_id":  replyID,
								"card_type": cardTypeFileCreate,
								"title":     "创建文件",
								"files": []map[string]string{
									{
										"path":     "workflow.yaml",
										"language": "yaml",
										"content":  trimmedYAML,
									},
								},
							})
							flusher.Flush()
						}
					}
				}
				if evt.Node == "fixer" && evt.Status == "done" {
					if yamlText, ok := evt.Data["yaml"].(string); ok {
						trimmedYAML := strings.TrimSpace(yamlText)
						if trimmedYAML != "" {
							writeSSE(w, "card", map[string]any{
								"card_id":   "file-create",
								"reply_id":  replyID,
								"card_type": cardTypeFileCreate,
								"title":     "创建文件",
								"files": []map[string]string{
									{
										"path":     "workflow.yaml",
										"language": "yaml",
										"content":  trimmedYAML,
									},
								},
							})
							flusher.Flush()
						}
					}
				}
				flusher.Flush()
				continue
			}
			events = nil
		case delta, ok := <-streamCh:
			if ok {
				wrote := false
				if delta.Thought != "" {
					sentReasoningDelta = true
					writeSSE(w, "delta", map[string]any{
						"channel": "reasoning",
						"content": delta.Thought,
					})
					wrote = true
				}
				if delta.Content != "" {
					writeSSE(w, "delta", map[string]any{
						"channel": "answer",
						"content": delta.Content,
					})
					wrote = true
				}
				if wrote {
					flusher.Flush()
				}
				continue
			}
			streamCh = nil
		case result := <-resultCh:
			if result.err != nil {
				writeSSE(w, "error", map[string]string{"error": result.err.Error()})
				flusher.Flush()
				return
			}
			pending = &result
			resultCh = nil
		case <-ctx.Done():
			return
		}

		if pending != nil && events == nil && streamCh == nil {
			if pending.state != nil && !sentReasoningDelta {
				streamReasoning(w, flusher, pending.state.Thought)
			}
			if pending.state != nil && pending.state.ExecutionResult != nil && !pending.state.ExecutionSkipped {
				execResult := pending.state.ExecutionResult
				s.recordValidationAudit(validationAuditEntry{
					Source:    "ai-workflow-stream",
					Env:       envName,
					EnvType:   envType,
					Status:    execResult.Status,
					Code:      execResult.Code,
					Error:     pending.state.LastError,
					YAMLHash:  hashYAML(pending.state.YAML),
					StepCount: countStepsInYAML(pending.state.YAML),
				})
			}
			title := ""
			prompt := ""
			if mode == string(aiworkflow.ModeGenerate) {
				title = titleFromMessage(req.Prompt)
				prompt = req.Prompt
			} else if mode == "migrate" {
				title = "AI Migrate"
				prompt = firstNonEmpty(req.Script, req.Prompt)
			} else if strings.TrimSpace(req.DraftID) == "" {
				title = "AI Fix"
			}
			draftID := ""
			if mode != "simulate" {
				draftID = s.saveAIDraft(req.DraftID, title, prompt, pending.state)
				if pending.state != nil {
					trimmedYAML := strings.TrimSpace(pending.state.YAML)
					if trimmedYAML != "" {
						path := "workflow.yaml"
						if strings.TrimSpace(draftID) != "" {
							path = fmt.Sprintf("workflows/%s.yaml", draftID)
						}
						writeSSE(w, "card", map[string]any{
							"card_id":   "file-create",
							"reply_id":  replyID,
							"card_type": cardTypeFileCreate,
							"title":     "创建文件",
							"files": []map[string]string{
								{
									"path":     path,
									"language": "yaml",
									"content":  trimmedYAML,
								},
							},
						})
					}
				}
			}
			payload := map[string]any{
				"yaml":         pending.state.YAML,
				"summary":      pending.state.Summary,
				"issues":       pending.state.Issues,
				"risk_level":   pending.state.RiskLevel,
				"needs_review": pending.state.NeedsReview,
				"questions":    pending.state.Questions,
				"history":      pending.state.History,
			}
			if mode != "simulate" {
				payload["draft_id"] = draftID
			}
			if pending.state.Intent != nil && pending.state.Intent.Type != "" {
				payload["intent_type"] = string(pending.state.Intent.Type)
			}
			if len(pending.state.Plan) > 0 {
				payload["plan"] = pending.state.Plan
			}
			if len(pending.state.SubagentSummaries) > 0 {
				payload["subagent_summaries"] = pending.state.SubagentSummaries
			}
			if pending.state.LoopMetrics != nil {
				payload["loop_metrics"] = map[string]any{
					"loop_id":       pending.state.LoopMetrics.LoopID,
					"iterations":    pending.state.LoopMetrics.Iterations,
					"tool_calls":    pending.state.LoopMetrics.ToolCalls,
					"tool_failures": pending.state.LoopMetrics.ToolFailures,
					"duration_ms":   pending.state.LoopMetrics.DurationMs,
				}
			}
			if pending.simulation != nil {
				payload["simulation"] = pending.simulation
			}
			if pending.migration != "" {
				payload["migration"] = map[string]any{"yaml": pending.migration}
			}
			logging.L().Debug("ai workflow stream result",
				zap.String("mode", mode),
				zap.Int("yaml_len", len(pending.state.YAML)),
				zap.Int("issues", len(pending.state.Issues)),
				zap.String("risk_level", string(pending.state.RiskLevel)),
				zap.Bool("needs_review", pending.state.NeedsReview),
				zap.String("draft_id", draftID),
			)
			writeSSE(w, "result", payload)
			flusher.Flush()
			return
		}
	}
}

func countStepsInYAML(yamlText string) int {
	lines := strings.Split(yamlText, "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- name:") {
			count++
		}
	}
	return count
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildExecuteDisplayName(displayName string) string {
	trimmed := strings.TrimSpace(displayName)
	if trimmed == "" {
		return ""
	}
	payload := map[string]string{
		"name_executing":      "正在" + trimmed,
		"name_executed":       "已完成" + trimmed,
		"name_execute_failed": trimmed + "失败",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func buildStreamMessageFromEvent(evt aiworkflow.Event, replyID string, index int) (streamMessage, bool) {
	if strings.TrimSpace(evt.Node) == "" {
		return streamMessage{}, false
	}
	callID := strings.TrimSpace(evt.CallID)
	if callID == "" {
		callID = evt.Node
	}
	displayName := strings.TrimSpace(evt.DisplayName)
	if displayName == "" {
		displayName = evt.Node
	}
	msgType := "tool_response"
	isFinish := true
	if evt.Status == "start" {
		msgType = "function_call"
		isFinish = false
	} else if evt.Status == "stream" {
		msgType = "tool_response"
		isFinish = false
	} else if evt.Status == "stream_finish" || evt.Status == "stream_done" {
		msgType = "tool_response"
		isFinish = true
	}
	pluginStatus := "0"
	if evt.Status == "error" {
		pluginStatus = "1"
	}
	streamPluginRunning := streamPluginRunningValue(evt)
	messageContent := evt.Message
	if strings.TrimSpace(messageContent) == "" && evt.Data != nil {
		if value, ok := evt.Data["tool_output_content"].(string); ok {
			messageContent = value
		} else if value, ok := evt.Data["content"].(string); ok {
			messageContent = value
		}
	}
	messageID := fmt.Sprintf("%s-%s-%d", callID, evt.Status, index)
	return streamMessage{
		MessageID: messageID,
		ReplyID:   replyID,
		Role:      "assistant",
		Type:      msgType,
		Content:   messageContent,
		IsFinish:  isFinish,
		Index:     index,
		ExtraInfo: streamMessageExt{
			CallID:              callID,
			ExecuteDisplayName:  buildExecuteDisplayName(displayName),
			PluginStatus:        pluginStatus,
			StreamPluginRunning: streamPluginRunning,
			LoopID:              evt.LoopID,
			Iteration:           evt.Iteration,
			AgentStatus:         evt.AgentStatus,
			AgentID:             evt.AgentID,
			AgentName:           evt.AgentName,
			AgentRole:           evt.AgentRole,
		},
	}, true
}

func buildStreamPluginFinishVerbose(evt aiworkflow.Event, replyID string, index int) (streamMessage, bool) {
	streamUUID := streamPluginRunningValue(evt)
	if streamUUID == "" {
		return streamMessage{}, false
	}
	toolOutput := ""
	if evt.Data != nil {
		if value, ok := evt.Data["tool_output_content"].(string); ok {
			toolOutput = value
		} else if value, ok := evt.Data["content"].(string); ok {
			toolOutput = value
		}
	}
	dataPayload := map[string]string{
		"uuid":                streamUUID,
		"tool_output_content": toolOutput,
	}
	dataBytes, _ := json.Marshal(dataPayload)
	contentPayload := map[string]string{
		"msg_type": "stream_plugin_finish",
		"data":     string(dataBytes),
	}
	contentBytes, _ := json.Marshal(contentPayload)
	callID := strings.TrimSpace(evt.CallID)
	if callID == "" {
		callID = evt.Node
	}
	return streamMessage{
		MessageID: fmt.Sprintf("%s-verbose-%d", callID, index),
		ReplyID:   replyID,
		Role:      "assistant",
		Type:      "verbose",
		Content:   string(contentBytes),
		IsFinish:  true,
		Index:     index,
		ExtraInfo: streamMessageExt{
			CallID:              callID,
			ExecuteDisplayName:  buildExecuteDisplayName(strings.TrimSpace(evt.DisplayName)),
			PluginStatus:        "0",
			StreamPluginRunning: streamUUID,
			LoopID:              evt.LoopID,
			Iteration:           evt.Iteration,
			AgentStatus:         evt.AgentStatus,
			AgentID:             evt.AgentID,
			AgentName:           evt.AgentName,
			AgentRole:           evt.AgentRole,
		},
	}, true
}

func streamPluginRunningValue(evt aiworkflow.Event) string {
	streamPluginRunning := strings.TrimSpace(evt.StreamPluginRunning)
	if streamPluginRunning == "" && evt.Data != nil {
		if value, ok := evt.Data["stream_plugin_running"].(string); ok {
			streamPluginRunning = strings.TrimSpace(value)
		} else if value, ok := evt.Data["stream_uuid"].(string); ok {
			streamPluginRunning = strings.TrimSpace(value)
		}
	}
	return streamPluginRunning
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{}`)
	}
	if event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", event)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func streamReasoning(w http.ResponseWriter, flusher http.Flusher, thought string) {
	trimmed := strings.TrimSpace(thought)
	if trimmed == "" {
		return
	}
	for _, chunk := range chunkRunes(trimmed, 12) {
		writeSSE(w, "delta", map[string]any{
			"channel": "reasoning",
			"content": chunk,
		})
		flusher.Flush()
		time.Sleep(18 * time.Millisecond)
	}
}

func chunkRunes(text string, size int) []string {
	if size <= 0 {
		return []string{text}
	}
	runes := []rune(text)
	chunks := make([]string, 0, (len(runes)+size-1)/size)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func (s *Server) saveAIDraft(id, title, prompt string, state *aiworkflow.State) string {
	if s.aiWorkflowStore == nil || state == nil {
		return id
	}
	draft := aiworkflowstore.Draft{
		ID:        id,
		Title:     title,
		Prompt:    prompt,
		YAML:      state.YAML,
		Summary:   state.Summary,
		Issues:    state.Issues,
		RiskLevel: string(state.RiskLevel),
	}
	saved, err := s.aiWorkflowStore.Save(draft)
	if err != nil {
		return id
	}
	return saved.ID
}

func (s *Server) defaultValidationEnv() *validationenv.ValidationEnv {
	if s.validationStore == nil {
		return nil
	}
	envs, err := s.validationStore.List()
	if err != nil || len(envs) == 0 {
		return nil
	}
	var picked string
	for _, env := range envs {
		if env.Type == validationenv.EnvTypeContainer {
			picked = env.Name
			break
		}
	}
	if picked == "" {
		picked = envs[0].Name
	}
	env, _, err := s.validationStore.Get(picked)
	if err != nil {
		return nil
	}
	return &env
}

func (s *Server) buildChatMessages(history []ai.Message, userMsg ai.Message) []ai.Message {
	messages := make([]ai.Message, 0, len(history)+2)
	contextText := s.buildContextText(nil)
	messages = append(messages, ai.Message{Role: "system", Content: s.systemPrompt(contextText)})

	trimmed := history
	if len(trimmed) > maxChatContextMessages {
		trimmed = trimmed[len(trimmed)-maxChatContextMessages:]
	}
	messages = append(messages, trimmed...)
	messages = append(messages, userMsg)
	return messages
}

func (s *Server) systemPrompt(contextText string) string {
	prompt := strings.TrimSpace(s.aiPrompt)
	if contextText == "" {
		return prompt
	}
	return strings.TrimSpace(fmt.Sprintf("%s\n\n上下文信息:\n%s", prompt, contextText))
}

func (s *Server) loopSystemPrompt(contextText string) string {
	prompt := strings.TrimSpace(s.aiLoopPrompt)
	if prompt == "" {
		prompt = strings.TrimSpace(s.aiPrompt)
	}
	if contextText == "" {
		return prompt
	}
	return strings.TrimSpace(fmt.Sprintf("%s\n\n上下文信息:\n%s", prompt, contextText))
}

func (s *Server) buildLoopToolExecutor(spec aiworkflow.AgentSpec) (aiworkflow.ToolExecutor, []string) {
	if s.skillRegistry == nil {
		return nil, nil
	}
	policy := parseToolConflictPolicy(s.cfg.ToolConflictPolicy)
	factory := skills.NewAgentFactory(s.skillRegistry, skills.WithToolConflictPolicy(policy))
	skillsRef := append([]string{}, spec.Skills...)
	if len(skillsRef) == 0 {
		skillsRef = s.resolveAgentSkills(spec.Name)
	}
	if len(skillsRef) == 0 {
		if strings.TrimSpace(spec.Name) != "" {
			logging.L().Warn("loop tools build skipped: agent has no skills", zap.String("agent", spec.Name))
			return nil, nil
		}
		skillsRef = append([]string{}, s.cfg.ClaudeSkills...)
	}
	if len(skillsRef) == 0 {
		return nil, nil
	}
	agentName := strings.TrimSpace(spec.Name)
	if agentName == "" {
		agentName = "loop"
	}
	bundle, err := factory.Build(skills.AgentSpec{
		Name:   agentName,
		Skills: skillsRef,
	})
	if err != nil {
		logging.L().Warn("loop tools build failed", zap.Error(err))
		return nil, nil
	}
	toolMap := make(map[string]tool.InvokableTool)
	names := make([]string, 0, len(bundle.Tools))
	for _, t := range bundle.Tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		name := strings.TrimSpace(info.Name)
		if name == "" {
			continue
		}
		toolMap[name] = t
		names = append(names, name)
	}
	if len(toolMap) == 0 {
		return nil, nil
	}
	sort.Strings(names)
	executor := func(ctx context.Context, name string, args map[string]any) (string, error) {
		toolItem, ok := toolMap[name]
		if !ok {
			return "", fmt.Errorf("tool not found: %s", name)
		}
		payload := encodeToolArgs(args)
		return toolItem.InvokableRun(ctx, payload)
	}
	return executor, names
}

func (s *Server) resolveAgentSkills(name string) []string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	for _, agent := range s.cfg.Agents {
		if strings.EqualFold(strings.TrimSpace(agent.Name), trimmed) {
			return append([]string{}, agent.Skills...)
		}
	}
	return nil
}

func parseToolConflictPolicy(raw string) skills.ToolConflictPolicy {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "overwrite":
		return skills.ToolConflictOverwrite
	case "keep":
		return skills.ToolConflictKeepExisting
	case "prefix":
		return skills.ToolConflictPrefix
	default:
		return skills.ToolConflictError
	}
}

func encodeToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	if string(data) == "null" {
		return "{}"
	}
	return string(data)
}

func buildAgentSpecs(primary string, agents []string) []aiworkflow.AgentSpec {
	seen := make(map[string]struct{})
	ordered := make([]string, 0)

	primaryName := strings.TrimSpace(primary)
	if primaryName != "" {
		ordered = append(ordered, primaryName)
		seen[primaryName] = struct{}{}
	}

	for _, name := range agents {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		ordered = append(ordered, trimmed)
	}

	if len(ordered) == 0 {
		ordered = append(ordered, "main")
	}

	if primaryName == "" {
		for i, name := range ordered {
			if name == "main" && i > 0 {
				ordered = append([]string{"main"}, append(ordered[:i], ordered[i+1:]...)...)
				break
			}
		}
	}

	specs := make([]aiworkflow.AgentSpec, 0, len(ordered))
	for _, name := range ordered {
		specs = append(specs, aiworkflow.AgentSpec{Name: name})
	}
	return specs
}

func (s *Server) buildContextText(extra map[string]any) string {
	parts := []string{}
	if s.envStore != nil {
		if envs, err := s.envStore.List(); err == nil && len(envs) > 0 {
			names := make([]string, 0, len(envs))
			for _, env := range envs {
				names = append(names, env.Name)
			}
			parts = append(parts, fmt.Sprintf("可用环境变量包: %s", strings.Join(names, ", ")))
		}
	}
	if s.scriptStore != nil {
		if scripts, err := s.scriptStore.List(); err == nil && len(scripts) > 0 {
			names := make([]string, 0, len(scripts))
			for _, script := range scripts {
				names = append(names, script.Name)
			}
			parts = append(parts, fmt.Sprintf("可用脚本库: %s", strings.Join(names, ", ")))
		}
	}
	if s.validationStore != nil {
		if envs, err := s.validationStore.List(); err == nil && len(envs) > 0 {
			names := make([]string, 0, len(envs))
			for _, env := range envs {
				names = append(names, env.Name)
			}
			parts = append(parts, fmt.Sprintf("可用验证环境: %s", strings.Join(names, ", ")))
		}
	}
	if len(extra) > 0 {
		if payload, err := json.MarshalIndent(extra, "", "  "); err == nil {
			parts = append(parts, "额外上下文(JSON):\n"+string(payload))
		}
	}
	return strings.Join(parts, "\n")
}

func buildFixPrompt(yaml string, issues []string) string {
	lines := []string{"下面 YAML 校验失败，请根据问题修复，并只输出修复后的 YAML：", "", yaml}
	if len(issues) > 0 {
		lines = append(lines, "", "问题列表:")
		for _, issue := range issues {
			lines = append(lines, "- "+issue)
		}
	}
	return strings.Join(lines, "\n")
}

func titleFromMessage(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "新会话"
	}
	runes := []rune(trimmed)
	if len(runes) > 12 {
		return string(runes[:12]) + "…"
	}
	return trimmed
}
