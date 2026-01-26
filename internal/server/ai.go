package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bops/internal/ai"
	"bops/internal/aistore"
	"bops/internal/aiworkflow"
	"bops/internal/aiworkflowstore"
	"bops/internal/logging"
	"bops/internal/validationenv"
	"bops/internal/validationrun"
	"bops/internal/workflow"
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
	Mode       string         `json:"mode,omitempty"`
	Prompt     string         `json:"prompt,omitempty"`
	YAML       string         `json:"yaml,omitempty"`
	Issues     []string       `json:"issues,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
	Env        string         `json:"env,omitempty"`
	Execute    bool           `json:"execute,omitempty"`
	MaxRetries int            `json:"max_retries,omitempty"`
	DraftID    string         `json:"draft_id,omitempty"`
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
	if mode == string(aiworkflow.ModeGenerate) && strings.TrimSpace(req.Prompt) == "" {
		writeError(w, r, http.StatusBadRequest, "prompt is required")
		return
	}
	if mode == string(aiworkflow.ModeFix) && strings.TrimSpace(req.YAML) == "" {
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
	envName := ""
	envType := ""
	if env != nil {
		envName = env.Name
		envType = string(env.Type)
	}
	logging.L().Debug("ai workflow stream start",
		zap.String("mode", mode),
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
		state *aiworkflow.State
		err   error
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
	opts := aiworkflow.RunOptions{
		SystemPrompt:  s.systemPrompt(contextText),
		ContextText:   contextText,
		ValidationEnv: env,
		SkipExecute:   !req.Execute,
		MaxRetries:    req.MaxRetries,
		EventSink:     sink,
		StreamSink:    streamSink,
		BaseYAML:      baseYAML,
	}

	go func() {
		defer close(events)
		defer close(streamCh)
		var state *aiworkflow.State
		var runErr error
		if mode == string(aiworkflow.ModeFix) {
			state, runErr = s.aiWorkflow.RunFix(ctx, req.YAML, req.Issues, opts)
		} else {
			state, runErr = s.aiWorkflow.RunGenerate(ctx, strings.TrimSpace(req.Prompt), req.Context, opts)
		}
		resultCh <- streamResult{state: state, err: runErr}
	}()

	var pending *streamResult
	sentThoughtDelta := false

	for {
		select {
		case evt, ok := <-events:
			if ok {
				writeSSE(w, "status", evt)
				flusher.Flush()
				continue
			}
			events = nil
		case delta, ok := <-streamCh:
			if ok {
				if delta.Thought != "" {
					sentThoughtDelta = true
					writeSSE(w, "delta", map[string]any{
						"channel": "thought",
						"content": delta.Thought,
					})
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
			if pending.state != nil && !sentThoughtDelta {
				streamThought(w, flusher, pending.state.Thought)
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
			} else if strings.TrimSpace(req.DraftID) == "" {
				title = "AI Fix"
			}
			draftID := s.saveAIDraft(req.DraftID, title, prompt, pending.state)
			payload := map[string]any{
				"yaml":         pending.state.YAML,
				"summary":      pending.state.Summary,
				"issues":       pending.state.Issues,
				"risk_level":   pending.state.RiskLevel,
				"needs_review": pending.state.NeedsReview,
				"questions":    pending.state.Questions,
				"history":      pending.state.History,
				"draft_id":     draftID,
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

func streamThought(w http.ResponseWriter, flusher http.Flusher, thought string) {
	trimmed := strings.TrimSpace(thought)
	if trimmed == "" {
		return
	}
	for _, chunk := range chunkRunes(trimmed, 12) {
		writeSSE(w, "delta", map[string]any{
			"channel": "thought",
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
