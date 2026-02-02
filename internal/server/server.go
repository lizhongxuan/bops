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
	"bops/internal/logging"
	"bops/internal/runmanager"
	"bops/internal/scriptstore"
	"bops/internal/skills"
	"bops/internal/state"
	"bops/internal/validationenv"
	"bops/internal/workflowstore"
	"go.uber.org/zap"
)

type Server struct {
	Addr            string
	StaticDir       string
	CORSOrigins     []string
	configPath      string
	cfg             config.Config
	mux             *http.ServeMux
	http            *http.Server
	store           *workflowstore.Store
	envStore        *envstore.Store
	aiStore         *aistore.Store
	aiClient        ai.Client
	aiPrompt        string
	aiLoopPrompt    string
	aiWorkflow      *aiworkflow.Pipeline
	aiWorkflowStore *aiworkflowstore.Store
	validationStore *validationenv.Store
	scriptStore     *scriptstore.Store
	engine          *engine.Engine
	runs            *runmanager.Manager
	bus             *eventbus.Bus
	auditLogPath    string
	skillLoader     *skills.Loader
	skillRegistry   *skills.Registry
}

func New(cfg config.Config, configPath string) *Server {
	if strings.TrimSpace(configPath) == "" {
		configPath = config.ResolvePath("")
	}
	mux := http.NewServeMux()
	bus := eventbus.New()
	aiClient, _ := ai.NewClient(ai.Config{
		Provider:      cfg.AIProvider,
		APIKey:        cfg.AIApiKey,
		BaseURL:       cfg.AIBaseURL,
		Model:         cfg.AIModel,
		PlannerModel:  cfg.AIPlannerModel,
		ExecutorModel: cfg.AIExecutorModel,
	})
	prompt := ai.LoadPrompt(filepath.Join("docs", "prompt-workflow.md"))
	loopPrompt := ai.LoadLoopPrompt(filepath.Join("docs", "prompt-loop.md"))
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
	logging.L().Debug("server init",
		zap.String("listen", cfg.ServerListen),
		zap.String("static_dir", cfg.StaticDir),
		zap.String("data_dir", cfg.DataDir),
		zap.String("state_path", cfg.StatePath),
		zap.Bool("ai_enabled", aiClient != nil),
		zap.String("config_path", configPath),
	)
	srv := &Server{
		Addr:            cfg.ServerListen,
		StaticDir:       cfg.StaticDir,
		CORSOrigins:     cfg.CORSOrigins,
		configPath:      configPath,
		cfg:             cfg,
		mux:             mux,
		store:           workflowstore.New(filepath.Join(cfg.DataDir, "workflows")),
		envStore:        envstore.New(filepath.Join(cfg.DataDir, "envs")),
		aiStore:         aistore.New(filepath.Join(cfg.DataDir, "ai_sessions")),
		aiClient:        aiClient,
		aiPrompt:        prompt,
		aiLoopPrompt:    loopPrompt,
		aiWorkflow:      aiWorkflow,
		aiWorkflowStore: aiWorkflowStore,
		validationStore: validationenv.NewStore(filepath.Join(cfg.DataDir, "validation_envs")),
		scriptStore:     scriptStore,
		engine:          engine.New(defaultRegistry(scriptStore)),
		runs:            runmanager.NewWithBus(state.NewFileStore(cfg.StatePath), bus),
		bus:             bus,
		auditLogPath:    filepath.Join(cfg.DataDir, "validation_audit.log"),
	}
	srv.initSkills(cfg)
	srv.routes()
	return srv
}

func (s *Server) ListenAndServe() error {
	if s.http == nil {
		s.http = &http.Server{
			Addr:              s.Addr,
			Handler:           s.withCORS(s.withLogging(s.mux)),
			ReadHeaderTimeout: 5 * time.Second,
		}
	}
	logging.L().Info("http server listening", zap.String("addr", s.Addr))
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	logging.L().Info("http server shutting down")
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

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.RequestURI()),
			zap.Int("status", status),
			zap.Duration("duration", time.Since(start)),
			zap.Int("bytes", recorder.bytes),
		}
		if status >= http.StatusInternalServerError {
			logging.L().Error("request failed", fields...)
		} else {
			logging.L().Debug("request handled", fields...)
		}
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
