package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"bops/internal/scriptstore"
)

type scriptListResponse struct {
	Items []scriptstore.Summary `json:"items"`
	Total int                   `json:"total"`
}

type scriptRequest struct {
	Name        string   `json:"name"`
	Language    string   `json:"language"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Content     string   `json:"content"`
}

func (s *Server) handleScripts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.scriptStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	if search != "" {
		filtered := items[:0]
		for _, item := range items {
			tagText := strings.ToLower(strings.Join(item.Tags, " "))
			if strings.Contains(strings.ToLower(item.Name), search) ||
				strings.Contains(strings.ToLower(item.Description), search) ||
				strings.Contains(tagText, search) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	writeJSON(w, http.StatusOK, scriptListResponse{Items: items, Total: len(items)})
}

func (s *Server) handleScript(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/scripts/")
	name := strings.Trim(path, "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "script name is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		script, _, err := s.scriptStore.Get(name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, script)
	case http.MethodPut:
		body, err := readBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var req scriptRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			req.Name = name
		}
		script, err := s.scriptStore.Put(name, scriptstore.Script{
			Name:        req.Name,
			Language:    req.Language,
			Description: req.Description,
			Tags:        req.Tags,
			Content:     req.Content,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, script)
	case http.MethodDelete:
		if err := s.scriptStore.Delete(name); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
