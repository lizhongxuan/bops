package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"bops/internal/ai"
	"bops/internal/aiworkflow"
	"bops/internal/logging"
	"bops/internal/workbench"
	"bops/internal/workflow"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type nodeRegenRequest struct {
	Node      aiworkflow.NodeSpec `json:"node"`
	Neighbors nodeRegenNeighbors  `json:"neighbors"`
	Workflow  nodeRegenWorkflow   `json:"workflow"`
	Intent    string              `json:"intent"`
	Context   map[string]any      `json:"context,omitempty"`
}

type nodeRegenNeighbors struct {
	Prev []aiworkflow.NeighborSpec `json:"prev"`
	Next []aiworkflow.NeighborSpec `json:"next"`
}

type nodeRegenWorkflow struct {
	YAML string `json:"yaml"`
}

type nodeRegenResponse struct {
	YAML      string                 `json:"yaml"`
	Graph     any                    `json:"graph,omitempty"`
	Node      aiworkflow.NodeSpec    `json:"node"`
	Questions []string               `json:"questions,omitempty"`
	Changes   []string               `json:"changes,omitempty"`
	Message   string                 `json:"message,omitempty"`
	DraftID   string                 `json:"draft_id,omitempty"`
	Summary   string                 `json:"summary,omitempty"`
	Issues    []string               `json:"issues,omitempty"`
	RiskLevel string                 `json:"risk_level,omitempty"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

func (s *Server) handleAIWorkflowNodeRegenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.aiClient == nil {
		writeError(w, r, http.StatusServiceUnavailable, "ai provider is not configured")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	var req nodeRegenRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json payload")
		return
	}
	yamlText := strings.TrimSpace(req.Workflow.YAML)
	if yamlText == "" {
		writeError(w, r, http.StatusBadRequest, "workflow yaml is required")
		return
	}
	logging.L().Info("ai workflow node regenerate request",
		zap.String("node_id", strings.TrimSpace(req.Node.ID)),
		zap.Int("node_index", req.Node.Index),
		zap.String("action", strings.TrimSpace(req.Node.Action)),
		zap.Int("yaml_len", len(yamlText)),
		zap.Int("intent_len", len(req.Intent)),
	)
	wf, err := workflow.Load([]byte(yamlText))
	if err != nil {
		logging.L().Warn("ai workflow node regenerate load failed", zap.Error(err))
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	stepIndex, ok := resolveStepIndex(req.Node, wf)
	if !ok {
		logging.L().Warn("ai workflow node regenerate resolve index failed",
			zap.String("node_id", strings.TrimSpace(req.Node.ID)),
			zap.String("node_name", strings.TrimSpace(req.Node.Name)),
		)
		writeError(w, r, http.StatusBadRequest, "unable to resolve node index")
		return
	}

	prompt := aiworkflow.BuildNodeRegeneratePrompt(req.Intent, req.Node, req.Neighbors.Prev, req.Neighbors.Next, yamlText)
	messages := []ai.Message{
		{Role: "system", Content: s.systemPrompt(s.buildContextText(req.Context))},
		{Role: "user", Content: prompt},
	}
	reply, err := s.aiClient.Chat(r.Context(), messages)
	if err != nil {
		logging.L().Error("ai workflow node regenerate chat failed", zap.Error(err))
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	regen, err := aiworkflow.ParseNodeRegenResponse(reply)
	if err != nil {
		logging.L().Warn("ai workflow node regenerate parse failed", zap.Error(err))
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	step, issues := aiworkflow.NormalizeNodeRegenStep(regen.Step)
	if len(issues) > 0 {
		logging.L().Warn("ai workflow node regenerate normalize issues", zap.Int("issues", len(issues)))
		writeError(w, r, http.StatusBadRequest, strings.Join(issues, "; "))
		return
	}
	if stepIndex < 0 || stepIndex >= len(wf.Steps) {
		writeError(w, r, http.StatusBadRequest, "node index out of range")
		return
	}

	previous := wf.Steps[stepIndex]
	step.Targets = previous.Targets
	step.When = previous.When
	step.Retries = previous.Retries
	step.Timeout = previous.Timeout
	step.Loop = previous.Loop
	step.Notify = previous.Notify
	wf.Steps[stepIndex] = step

	data, err := yaml.Marshal(wf)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	updatedYAML := strings.TrimSpace(string(data))

	graphPayload := any(nil)
	if graph, err := workbench.GraphFromYAML(updatedYAML); err == nil {
		if data, err := json.Marshal(graph); err == nil {
			var raw any
			if err := json.Unmarshal(data, &raw); err == nil {
				graphPayload = raw
			}
		}
	}

	changes := diffStepChanges(previous, step)
	logging.L().Info("ai workflow node regenerate done",
		zap.String("node_id", strings.TrimSpace(req.Node.ID)),
		zap.Int("node_index", stepIndex),
		zap.Int("changes", len(changes)),
		zap.Int("yaml_len", len(updatedYAML)),
	)
	resp := nodeRegenResponse{
		YAML:      updatedYAML,
		Graph:     graphPayload,
		Node:      aiworkflow.NodeSpec{ID: req.Node.ID, Index: stepIndex, Name: step.Name, Action: step.Action, With: step.With, Targets: step.Targets},
		Questions: regen.Questions,
		Changes:   changes,
		Message:   "节点已更新",
	}
	writeJSON(w, http.StatusOK, resp)
}

func resolveStepIndex(node aiworkflow.NodeSpec, wf workflow.Workflow) (int, bool) {
	if node.Index >= 0 {
		return node.Index, node.Index < len(wf.Steps)
	}
	id := strings.TrimSpace(node.ID)
	if strings.HasPrefix(id, "step-") {
		var idx int
		if _, err := fmt.Sscanf(id, "step-%d", &idx); err == nil {
			idx = idx - 1
			if idx >= 0 && idx < len(wf.Steps) {
				return idx, true
			}
		}
	}
	name := strings.TrimSpace(node.Name)
	if name != "" {
		found := -1
		for i, step := range wf.Steps {
			if step.Name == name {
				if found >= 0 {
					return -1, false
				}
				found = i
			}
		}
		if found >= 0 {
			return found, true
		}
	}
	return -1, false
}

func diffStepChanges(prev, next workflow.Step) []string {
	changes := make([]string, 0, 4)
	if strings.TrimSpace(prev.Name) != strings.TrimSpace(next.Name) {
		changes = append(changes, "name")
	}
	if strings.TrimSpace(prev.Action) != strings.TrimSpace(next.Action) {
		changes = append(changes, "action")
	}
	if !deepEqualJSON(prev.With, next.With) {
		changes = append(changes, "with")
	}
	return changes
}

func deepEqualJSON(a, b any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}
