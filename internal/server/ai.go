package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"bops/internal/ai"
	"bops/internal/aistore"
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
}

type aiChatResponse struct {
	Reply   ai.Message      `json:"reply"`
	Session aistore.Session `json:"session"`
}

type aiGenerateRequest struct {
	Prompt  string         `json:"prompt"`
	Context map[string]any `json:"context,omitempty"`
}

type aiGenerateResponse struct {
	YAML    string `json:"yaml"`
	Message string `json:"message,omitempty"`
}

type aiFixRequest struct {
	YAML   string   `json:"yaml"`
	Issues []string `json:"issues,omitempty"`
}

func (s *Server) handleAIChatSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.aiStore.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, aiSessionListResponse{Items: items, Total: len(items)})
	case http.MethodPost:
		body, err := readBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var req aiSessionCreateRequest
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json payload")
				return
			}
		}
		session, err := s.aiStore.Create(req.Title)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, aiSessionResponse{Session: session})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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
		writeError(w, http.StatusNotFound, "session id is required")
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	session, _, err := s.aiStore.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, aiSessionResponse{Session: session})
}

func (s *Server) handleAIChatMessages(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id = strings.Trim(id, "/")
	if id == "" {
		writeError(w, http.StatusNotFound, "session id is required")
		return
	}
	if s.aiClient == nil {
		writeError(w, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req aiChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	session, _, err := s.aiStore.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	userMsg := ai.Message{Role: "user", Content: strings.TrimSpace(req.Content)}
	messages := s.buildChatMessages(session.Messages, userMsg)

	reply, err := s.aiClient.Chat(r.Context(), messages)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	assistantMsg := ai.Message{Role: "assistant", Content: strings.TrimSpace(reply)}
	session.Messages = append(session.Messages, userMsg, assistantMsg)
	if session.Title == "新会话" && strings.TrimSpace(session.Messages[0].Content) != "" {
		session.Title = titleFromMessage(session.Messages[0].Content)
	}
	if err := s.aiStore.Save(session); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, aiChatResponse{Reply: assistantMsg, Session: session})
}

func (s *Server) handleAIWorkflowGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.aiClient == nil {
		writeError(w, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req aiGenerateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	contextText := s.buildContextText(req.Context)
	messages := []ai.Message{{
		Role:    "system",
		Content: s.systemPrompt(contextText),
	}, {
		Role:    "user",
		Content: strings.TrimSpace(req.Prompt),
	}}

	reply, err := s.aiClient.Chat(r.Context(), messages)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	yaml := ai.ExtractYAML(reply)
	writeJSON(w, http.StatusOK, aiGenerateResponse{YAML: yaml})
}

func (s *Server) handleAIWorkflowFix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.aiClient == nil {
		writeError(w, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req aiFixRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		writeError(w, http.StatusBadRequest, "yaml is required")
		return
	}

	prompt := buildFixPrompt(req.YAML, req.Issues)
	messages := []ai.Message{{
		Role:    "system",
		Content: s.systemPrompt(""),
	}, {
		Role:    "user",
		Content: prompt,
	}}

	reply, err := s.aiClient.Chat(r.Context(), messages)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	yaml := ai.ExtractYAML(reply)
	writeJSON(w, http.StatusOK, aiGenerateResponse{YAML: yaml})
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
