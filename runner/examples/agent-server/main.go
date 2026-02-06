package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bops/runner/logging"
	"bops/runner/modules"
	"bops/runner/modules/cmd"
	"bops/runner/modules/shell"
	"bops/runner/scheduler"
	"go.uber.org/zap"
)

type runRequest struct {
	Task scheduler.Task `json:"task"`
}

type runResponse struct {
	Result scheduler.Result `json:"result"`
	Error  string           `json:"error,omitempty"`
}

type statusRequest struct {
	TaskID string `json:"task_id"`
}

type taskEntry struct {
	Result     scheduler.Result
	Done       bool
	StartedAt  time.Time
	FinishedAt time.Time
	Cancel     context.CancelFunc
	Canceled   bool
	Stdout     *outputBuffer
	Stderr     *outputBuffer
}

type outputBuffer struct {
	mu      sync.Mutex
	maxSize int
	data    []byte
}

func newOutputBuffer(maxSize int) *outputBuffer {
	return &outputBuffer{maxSize: maxSize}
}

func (b *outputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if b.maxSize > 0 && len(b.data) > b.maxSize {
		b.data = b.data[len(b.data)-b.maxSize:]
	}
	return len(p), nil
}

func (b *outputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func main() {
	fs := flag.NewFlagSet("agent-server", flag.ExitOnError)
	addr := fs.String("addr", ":7072", "listen address")
	token := fs.String("token", "runner-token", "auth token")
	logLevel := fs.String("log-level", "info", "log level (debug/info/warn/error)")
	logFormat := fs.String("log-format", "console", "log format (console/json)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if _, err := logging.Init(logging.Config{LogLevel: *logLevel, LogFormat: *logFormat}); err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}

	reg := modules.NewRegistry()
	_ = reg.Register("cmd.run", cmd.New())
	_ = reg.Register("shell.run", shell.New())

	var lastBeat atomic.Int64
	lastBeat.Store(time.Now().UTC().Unix())
	const asyncThreshold = 4 * time.Second

	var taskMu sync.Mutex
	tasks := map[string]*taskEntry{}

	getTask := func(id string) (*taskEntry, bool) {
		taskMu.Lock()
		defer taskMu.Unlock()
		entry, ok := tasks[id]
		return entry, ok
	}

	setTask := func(id string, entry *taskEntry) {
		taskMu.Lock()
		defer taskMu.Unlock()
		tasks[id] = entry
	}

	updateTask := func(id string, result scheduler.Result, done bool) {
		taskMu.Lock()
		defer taskMu.Unlock()
		entry, ok := tasks[id]
		if !ok {
			entry = &taskEntry{}
			tasks[id] = entry
		}
		entry.Result = result
		entry.Done = done
		if done {
			entry.FinishedAt = time.Now().UTC()
		}
	}

	cancelTask := func(id string) bool {
		taskMu.Lock()
		defer taskMu.Unlock()
		entry, ok := tasks[id]
		if !ok || entry.Done {
			return false
		}
		if entry.Cancel != nil {
			entry.Cancel()
		}
		entry.Canceled = true
		entry.Result = scheduler.Result{TaskID: id, Status: "canceled"}
		entry.Done = true
		entry.FinishedAt = time.Now().UTC()
		return true
	}

	checkAuth := func(w http.ResponseWriter, r *http.Request) bool {
		trimmed := strings.TrimSpace(*token)
		if trimmed == "" {
			return true
		}
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			auth = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}
		headerToken := strings.TrimSpace(r.Header.Get("X-Runner-Token"))
		if auth == trimmed || headerToken == trimmed {
			return true
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
		return false
	}

	http.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !checkAuth(w, r) {
			return
		}
		var req runRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(runResponse{Error: err.Error()})
			return
		}
		if strings.TrimSpace(req.Task.ID) == "" {
			req.Task.ID = fmt.Sprintf("task-%d", time.Now().UTC().UnixNano())
		}

		if existing, ok := getTask(req.Task.ID); ok {
			if existing.Done {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(runResponse{Result: existing.Result, Error: existing.Result.Error})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(runResponse{Result: scheduler.Result{TaskID: req.Task.ID, Status: "running"}})
			return
		}
		logging.L().Info("agent run start",
			zap.String("task_id", req.Task.ID),
			zap.String("step", req.Task.Step.Name),
			zap.String("action", req.Task.Step.Action),
			zap.String("host", req.Task.Host.Name),
		)

		module, ok := reg.Get(req.Task.Step.Action)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(runResponse{Error: "unsupported action"})
			return
		}

		outputLimit := 65536
		if req.Task.Step.Args != nil {
			if raw, ok := req.Task.Step.Args["max_output_bytes"]; ok {
				switch v := raw.(type) {
				case int:
					outputLimit = v
				case int64:
					outputLimit = int(v)
				case float64:
					outputLimit = int(v)
				case string:
					var out int
					_, _ = fmt.Sscanf(strings.TrimSpace(v), "%d", &out)
					if out > 0 {
						outputLimit = out
					}
				}
			}
		}

		entry := &taskEntry{
			Result:    scheduler.Result{TaskID: req.Task.ID, Status: "running"},
			StartedAt: time.Now().UTC(),
			Stdout:    newOutputBuffer(outputLimit),
			Stderr:    newOutputBuffer(outputLimit),
		}
		setTask(req.Task.ID, entry)

		doneCh := make(chan scheduler.Result, 1)
		go func() {
			runCtx, cancel := context.WithCancel(context.Background())
			entry.Cancel = cancel
			res, err := module.Apply(runCtx, modules.Request{
				Step:   req.Task.Step,
				Host:   req.Task.Host,
				Vars:   req.Task.Vars,
				Stdout: entry.Stdout,
				Stderr: entry.Stderr,
			})

			result := scheduler.Result{TaskID: req.Task.ID, Status: "success", Output: res.Output}
			if err != nil {
				if runCtx.Err() != nil {
					result.Status = "canceled"
					result.Error = runCtx.Err().Error()
				} else {
					result.Status = "failed"
					result.Error = err.Error()
				}
			}
			updateTask(req.Task.ID, result, true)
			doneCh <- result
		}()

		select {
		case result := <-doneCh:
			updateTask(req.Task.ID, result, true)
			logging.L().Info("agent run finish",
				zap.String("task_id", req.Task.ID),
				zap.String("status", result.Status),
			)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(runResponse{Result: result, Error: result.Error})
		case <-time.After(asyncThreshold):
			logging.L().Info("agent run switched to async",
				zap.String("task_id", req.Task.ID),
				zap.Duration("threshold", asyncThreshold),
			)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(runResponse{Result: scheduler.Result{TaskID: req.Task.ID, Status: "running"}})
		}
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !checkAuth(w, r) {
			return
		}
		taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
		if taskID == "" && r.Method == http.MethodPost {
			var req statusRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(runResponse{Error: err.Error()})
				return
			}
			taskID = strings.TrimSpace(req.TaskID)
		}
		if taskID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(runResponse{Error: "task_id is required"})
			return
		}
		entry, ok := getTask(taskID)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(runResponse{Error: "task not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if entry.Done {
			_ = json.NewEncoder(w).Encode(runResponse{Result: entry.Result, Error: entry.Result.Error})
			return
		}
		result := scheduler.Result{TaskID: taskID, Status: "running"}
		if entry.Stdout != nil || entry.Stderr != nil {
			result.Output = map[string]any{
				"stdout": "",
				"stderr": "",
			}
			if entry.Stdout != nil {
				result.Output["stdout"] = entry.Stdout.String()
			}
			if entry.Stderr != nil {
				result.Output["stderr"] = entry.Stderr.String()
			}
		}
		_ = json.NewEncoder(w).Encode(runResponse{Result: result})
	})

	http.HandleFunc("/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !checkAuth(w, r) {
			return
		}
		taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
		if taskID == "" && r.Method == http.MethodPost {
			var req statusRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(runResponse{Error: err.Error()})
				return
			}
			taskID = strings.TrimSpace(req.TaskID)
		}
		if taskID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(runResponse{Error: "task_id is required"})
			return
		}
		if !cancelTask(taskID) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(runResponse{Error: "task not found or already done"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(runResponse{Result: scheduler.Result{TaskID: taskID, Status: "canceled"}})
	})

	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !checkAuth(w, r) {
			return
		}
		now := time.Now().UTC()
		lastBeat.Store(now.Unix())
		logging.L().Debug("agent heartbeat", zap.String("addr", *addr))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     "ok",
			"last_beat":  now.Format(time.RFC3339),
			"timestamp":  now.Unix(),
			"capability": []string{"cmd.run"},
		})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		now := time.Now().UTC()
		last := time.Unix(lastBeat.Load(), 0)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"timestamp": now.Unix(),
			"last_beat": last.Format(time.RFC3339),
		})
	})

	logging.L().Info("agent server listening",
		zap.String("addr", *addr),
		zap.Bool("token_required", strings.TrimSpace(*token) != ""),
	)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
