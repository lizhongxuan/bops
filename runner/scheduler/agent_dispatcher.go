package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"bops/runner/logging"
	"go.uber.org/zap"
)

type AgentDispatcher struct {
	BaseURL string
	Client  *http.Client
	Headers map[string]string
	Token   string
	// Heartbeat enables a pre-flight heartbeat call before each dispatch.
	Heartbeat bool
	// HeartbeatPath overrides the default heartbeat endpoint path.
	HeartbeatPath string
	// StatusPath overrides the default status endpoint path.
	StatusPath string
	// RetryMax defines how many retries after the initial attempt.
	RetryMax int
	// RetryDelay defines the delay between retries.
	RetryDelay time.Duration
	// DispatchTimeout overrides the http client timeout when Client is nil.
	DispatchTimeout time.Duration
	// HeartbeatTimeout overrides the heartbeat http client timeout when Client is nil.
	HeartbeatTimeout time.Duration
	// AsyncTimeout controls how long to wait for async task completion.
	AsyncTimeout time.Duration
	// PollInterval controls how often to poll /status.
	PollInterval time.Duration
	// OnOutput receives streaming output chunks while polling async tasks.
	OnOutput      func(taskID, step, host, stream, chunk string)
	outputMu      sync.Mutex
	outputOffsets map[string]outputOffset
	taskMetaMu    sync.Mutex
	taskMeta      map[string]taskMeta
}

type outputOffset struct {
	stdout int
	stderr int
}

type taskMeta struct {
	step string
	host string
}

func NewAgentDispatcher(baseURL string) *AgentDispatcher {
	return &AgentDispatcher{
		BaseURL:       strings.TrimSpace(baseURL),
		Client:        &http.Client{Timeout: 30 * time.Second},
		outputOffsets: map[string]outputOffset{},
		taskMeta:      map[string]taskMeta{},
	}
}

func NewAgentDispatcherWithToken(baseURL, token string) *AgentDispatcher {
	dispatcher := NewAgentDispatcher(baseURL)
	dispatcher.Token = strings.TrimSpace(token)
	return dispatcher
}

func (d *AgentDispatcher) Dispatch(ctx context.Context, task Task) (Result, error) {
	if d == nil {
		return Result{}, fmt.Errorf("agent dispatcher is nil")
	}
	d.setTaskMeta(task.ID, task.Step.Name, task.Host.Name)
	defer d.clearTaskMeta(task.ID)
	baseURL := strings.TrimSpace(d.BaseURL)
	if strings.TrimSpace(task.Host.Address) != "" {
		baseURL = strings.TrimSpace(task.Host.Address)
	}
	if baseURL == "" {
		return Result{}, fmt.Errorf("agent dispatcher base url is required")
	}
	attempts := d.RetryMax + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	var lastResult Result

	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			return lastResult, ctx.Err()
		}
		if d.Heartbeat {
			if err := d.sendHeartbeat(ctx, baseURL); err != nil {
				lastErr = err
				if attempt < attempts-1 {
					logging.L().Warn("agent heartbeat failed, retrying",
						zap.String("host", baseURL),
						zap.Int("attempt", attempt+1),
						zap.Error(err),
					)
					if waitErr := d.sleepWithContext(ctx); waitErr != nil {
						return lastResult, waitErr
					}
					continue
				}
				return Result{}, err
			}
		}

		result, err := d.dispatchOnce(ctx, baseURL, task)
		if err == nil {
			if strings.EqualFold(result.Status, "running") {
				return d.pollStatus(ctx, baseURL, result.TaskID)
			}
			return result, nil
		}
		lastErr = err
		lastResult = result
		if attempt < attempts-1 {
			logging.L().Warn("agent dispatch failed, retrying",
				zap.String("task_id", task.ID),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			if waitErr := d.sleepWithContext(ctx); waitErr != nil {
				return lastResult, waitErr
			}
		}
	}
	if lastErr != nil {
		return lastResult, lastErr
	}
	return lastResult, fmt.Errorf("agent dispatch failed")
}

var _ Dispatcher = (*AgentDispatcher)(nil)

func (d *AgentDispatcher) sendHeartbeat(ctx context.Context, baseURL string) error {
	path := strings.TrimSpace(d.HeartbeatPath)
	if path == "" {
		path = "/heartbeat"
	}
	url := strings.TrimRight(baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	if token := strings.TrimSpace(d.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Runner-Token", token)
	}
	client := d.clientWithTimeout(d.HeartbeatTimeout, 10*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := readLimitedBody(resp.Body)
		logging.L().Warn("agent heartbeat failed",
			zap.String("status", resp.Status),
			zap.String("body", body),
			zap.String("url", url),
		)
		if body != "" {
			return fmt.Errorf("agent heartbeat failed: %s (%s)", resp.Status, body)
		}
		return fmt.Errorf("agent heartbeat failed: %s", resp.Status)
	}
	return nil
}

func (d *AgentDispatcher) dispatchOnce(ctx context.Context, baseURL string, task Task) (Result, error) {
	url := strings.TrimRight(baseURL, "/") + "/run"
	payload := struct {
		Task Task `json:"task"`
	}{Task: task}

	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(d.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Runner-Token", token)
	}
	for k, v := range d.Headers {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	client := d.clientWithTimeout(d.DispatchTimeout, 30*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := readLimitedBody(resp.Body)
		logging.L().Warn("agent dispatch failed",
			zap.String("status", resp.Status),
			zap.String("body", body),
			zap.String("url", url),
			zap.String("task_id", task.ID),
		)
		if body != "" {
			return Result{}, fmt.Errorf("agent dispatch failed: %s (%s)", resp.Status, body)
		}
		return Result{}, fmt.Errorf("agent dispatch failed: %s", resp.Status)
	}

	var decoded struct {
		Result Result `json:"result"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return Result{}, err
	}
	if decoded.Result.TaskID == "" {
		decoded.Result.TaskID = task.ID
	}
	if decoded.Error != "" {
		decoded.Result.Status = "failed"
		decoded.Result.Error = decoded.Error
		return decoded.Result, fmt.Errorf("%s", decoded.Error)
	}
	return decoded.Result, nil
}

func (d *AgentDispatcher) pollStatus(ctx context.Context, baseURL, taskID string) (Result, error) {
	if strings.TrimSpace(taskID) == "" {
		return Result{}, fmt.Errorf("task_id is required for status polling")
	}
	timeout := d.AsyncTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	interval := d.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		result, err := d.fetchStatus(ctx, baseURL, taskID)
		if err != nil {
			return Result{}, err
		}
		d.emitOutputDelta(taskID, result.Output)
		if !strings.EqualFold(result.Status, "running") {
			if result.Status == "" {
				result.Status = "success"
			}
			return result, nil
		}
		if err := sleepWithContextFor(ctx, interval); err != nil {
			return Result{}, err
		}
	}
}

func (d *AgentDispatcher) emitOutputDelta(taskID string, output map[string]any) {
	if len(output) == 0 {
		return
	}
	stdout := ""
	stderr := ""
	if v, ok := output["stdout"]; ok {
		stdout = fmt.Sprint(v)
	}
	if v, ok := output["stderr"]; ok {
		stderr = fmt.Sprint(v)
	}
	if stdout == "" && stderr == "" {
		return
	}
	d.outputMu.Lock()
	offset := d.outputOffsets[taskID]
	d.outputMu.Unlock()

	meta := d.getTaskMeta(taskID)

	if len(stdout) > offset.stdout {
		chunk := stdout[offset.stdout:]
		if strings.TrimSpace(chunk) != "" {
			logging.L().Info("agent output",
				zap.String("task_id", taskID),
				zap.String("stream", "stdout"),
				zap.String("chunk", chunk),
			)
			if d.OnOutput != nil {
				d.OnOutput(taskID, meta.step, meta.host, "stdout", chunk)
			}
		}
		offset.stdout = len(stdout)
	}
	if len(stderr) > offset.stderr {
		chunk := stderr[offset.stderr:]
		if strings.TrimSpace(chunk) != "" {
			logging.L().Info("agent output",
				zap.String("task_id", taskID),
				zap.String("stream", "stderr"),
				zap.String("chunk", chunk),
			)
			if d.OnOutput != nil {
				d.OnOutput(taskID, meta.step, meta.host, "stderr", chunk)
			}
		}
		offset.stderr = len(stderr)
	}

	d.outputMu.Lock()
	d.outputOffsets[taskID] = offset
	d.outputMu.Unlock()
}

func (d *AgentDispatcher) setTaskMeta(taskID, step, host string) {
	if strings.TrimSpace(taskID) == "" {
		return
	}
	d.taskMetaMu.Lock()
	defer d.taskMetaMu.Unlock()
	d.taskMeta[taskID] = taskMeta{step: step, host: host}
}

func (d *AgentDispatcher) getTaskMeta(taskID string) taskMeta {
	d.taskMetaMu.Lock()
	defer d.taskMetaMu.Unlock()
	return d.taskMeta[taskID]
}

func (d *AgentDispatcher) clearTaskMeta(taskID string) {
	if strings.TrimSpace(taskID) == "" {
		return
	}
	d.outputMu.Lock()
	delete(d.outputOffsets, taskID)
	d.outputMu.Unlock()

	d.taskMetaMu.Lock()
	delete(d.taskMeta, taskID)
	d.taskMetaMu.Unlock()
}

func (d *AgentDispatcher) fetchStatus(ctx context.Context, baseURL, taskID string) (Result, error) {
	path := strings.TrimSpace(d.StatusPath)
	if path == "" {
		path = "/status"
	}
	url := strings.TrimRight(baseURL, "/") + path
	payload := struct {
		TaskID string `json:"task_id"`
	}{TaskID: taskID}

	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(d.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Runner-Token", token)
	}
	for k, v := range d.Headers {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	client := d.clientWithTimeout(d.DispatchTimeout, 30*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := readLimitedBody(resp.Body)
		if body != "" {
			return Result{}, fmt.Errorf("agent status failed: %s (%s)", resp.Status, body)
		}
		return Result{}, fmt.Errorf("agent status failed: %s", resp.Status)
	}

	var decoded struct {
		Result Result `json:"result"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return Result{}, err
	}
	if decoded.Result.TaskID == "" {
		decoded.Result.TaskID = taskID
	}
	if decoded.Error != "" {
		decoded.Result.Status = "failed"
		decoded.Result.Error = decoded.Error
		return decoded.Result, fmt.Errorf("%s", decoded.Error)
	}
	return decoded.Result, nil
}

func (d *AgentDispatcher) clientWithTimeout(timeout, fallback time.Duration) *http.Client {
	if d.Client != nil {
		return d.Client
	}
	if timeout <= 0 {
		timeout = fallback
	}
	return &http.Client{Timeout: timeout}
}

func (d *AgentDispatcher) sleepWithContext(ctx context.Context) error {
	delay := d.RetryDelay
	if delay <= 0 {
		delay = time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func sleepWithContextFor(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func readLimitedBody(reader io.Reader) string {
	if reader == nil {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(reader, 2048))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}
