package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"bops/internal/nodetemplate"
)

type nodeTemplateListResponse struct {
	Items []nodetemplate.Summary `json:"items"`
	Total int                    `json:"total"`
}

type nodeTemplateRequest struct {
	Name        string                `json:"name"`
	Category    string                `json:"category"`
	Description string                `json:"description"`
	Tags        []string              `json:"tags"`
	Node        nodetemplate.NodeSpec `json:"node"`
}

func (s *Server) handleNodeTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	items, err := s.nodeTemplates.List()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	if search != "" {
		filtered := items[:0]
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Name), search) ||
				strings.Contains(strings.ToLower(item.Description), search) ||
				strings.Contains(strings.ToLower(item.Category), search) ||
				strings.Contains(strings.ToLower(item.Action), search) ||
				strings.Contains(strings.ToLower(strings.Join(item.Tags, " ")), search) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, nodeTemplateListResponse{Items: items, Total: len(items)})
}

func (s *Server) handleNodeTemplate(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/node-templates/")
	name := strings.Trim(path, "/")
	if name == "" {
		writeError(w, r, http.StatusNotFound, "template name is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		tpl, _, err := s.nodeTemplates.Get(name)
		if err != nil {
			writeError(w, r, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tpl)
	case http.MethodPut:
		body, err := readBody(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		var req nodeTemplateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			req.Name = name
		}
		tpl, err := s.nodeTemplates.Put(name, nodetemplate.Template{
			Name:        req.Name,
			Category:    req.Category,
			Description: req.Description,
			Tags:        req.Tags,
			Node:        req.Node,
		})
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tpl)
	case http.MethodDelete:
		if err := s.nodeTemplates.Delete(name); err != nil {
			writeError(w, r, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}
