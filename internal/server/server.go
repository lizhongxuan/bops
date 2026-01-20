package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bops/internal/ai"
	"bops/internal/aistore"
	"bops/internal/aiworkflow"
	"bops/internal/aiworkflowstore"
	"bops/internal/config"
	"bops/internal/engine"
	"bops/internal/envstore"
	"bops/internal/eventbus"
	"bops/internal/runmanager"
	"bops/internal/scriptstore"
	"bops/internal/state"
	"bops/internal/validationenv"
	"bops/internal/workflowstore"
)

type Server struct {
	Addr            string
	StaticDir       string
	CORSOrigins     []string
	mux             *http.ServeMux
	http            *http.Server
	store           *workflowstore.Store
	envStore        *envstore.Store
	aiStore         *aistore.Store
	aiClient        ai.Client
	aiPrompt        string
	aiWorkflow      *aiworkflow.Pipeline
	aiWorkflowStore *aiworkflowstore.Store
	validationStore *validationenv.Store
	scriptStore     *scriptstore.Store
	engine          *engine.Engine
	runs            *runmanager.Manager
	bus             *eventbus.Bus
	auditLogPath    string
}

func New(cfg config.Config) *Server {
	mux := http.NewServeMux()
	bus := eventbus.New()
	aiClient, _ := ai.NewClient(ai.Config{
		Provider: cfg.AIProvider,
		APIKey:   cfg.AIApiKey,
		BaseURL:  cfg.AIBaseURL,
		Model:    cfg.AIModel,
	})
	prompt := ai.LoadPrompt(filepath.Join("docs", "prompt-workflow.md"))
	scriptStore := scriptstore.New(filepath.Join(cfg.DataDir, "scripts"))
	aiWorkflowStore := aiworkflowstore.New(filepath.Join(cfg.DataDir, "ai_workflows"))
	var aiWorkflow *aiworkflow.Pipeline
	if aiClient != nil {
		aiWorkflow, _ = aiworkflow.New(aiworkflow.Config{
			Client:       aiClient,
			SystemPrompt: prompt,
			MaxRetries:   2,
		})
	}
	srv := &Server{
		Addr:            cfg.ServerListen,
		StaticDir:       cfg.StaticDir,
		CORSOrigins:     cfg.CORSOrigins,
		mux:             mux,
		store:           workflowstore.New(filepath.Join(cfg.DataDir, "workflows")),
		envStore:        envstore.New(filepath.Join(cfg.DataDir, "envs")),
		aiStore:         aistore.New(filepath.Join(cfg.DataDir, "ai_sessions")),
		aiClient:        aiClient,
		aiPrompt:        prompt,
		aiWorkflow:      aiWorkflow,
		aiWorkflowStore: aiWorkflowStore,
		validationStore: validationenv.NewStore(filepath.Join(cfg.DataDir, "validation_envs")),
		scriptStore:     scriptStore,
		engine:          engine.New(defaultRegistry(scriptStore)),
		runs:            runmanager.NewWithBus(state.NewFileStore(cfg.StatePath), bus),
		bus:             bus,
		auditLogPath:    filepath.Join(cfg.DataDir, "validation_audit.log"),
	}
	srv.routes()
	return srv
}

func (s *Server) ListenAndServe() error {
	if s.http == nil {
		s.http = &http.Server{
			Addr:              s.Addr,
			Handler:           s.withCORS(s.mux),
			ReadHeaderTimeout: 5 * time.Second,
		}
	}
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowOrigin := ""
		if origin != "" {
			if containsOrigin(s.CORSOrigins, "*") {
				allowOrigin = "*"
			} else if containsOrigin(s.CORSOrigins, origin) {
				allowOrigin = origin
			}
		}

		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func containsOrigin(origins []string, value string) bool {
	for _, origin := range origins {
		if strings.EqualFold(origin, value) {
			return true
		}
	}
	return false
}

func spaHandler(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	indexPath := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		clean := filepath.Clean(r.URL.Path)
		target := filepath.Join(dir, clean)
		info, err := os.Stat(target)
		if err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
}
