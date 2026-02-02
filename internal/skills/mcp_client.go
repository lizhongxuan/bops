package skills

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type mcpToolsResult struct {
	Tools []MCPToolDefinition `json:"tools"`
}

type mcpCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type MCPClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	mu      sync.Mutex
	nextID  int
	pending map[int]chan responseResult
}

func NewMCPClient(ctx context.Context, command string, args []string, workDir string) (*MCPClient, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = new(bytes.Buffer)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	client := &MCPClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		nextID:  1,
		pending: make(map[int]chan responseResult),
	}
	go client.readLoop()
	if err := client.initialize(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *MCPClient) Close() error {
	if c == nil {
		return nil
	}
	c.failPending(fmt.Errorf("mcp client closed"))
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

func (c *MCPClient) ListTools(ctx context.Context) ([]MCPToolDefinition, error) {
	var result mcpToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	return c.callToolWithRetry(ctx, name, args, 1)
}

func (c *MCPClient) callToolWithRetry(ctx context.Context, name string, args map[string]any, retries int) (string, error) {
	payload := map[string]any{
		"name":      name,
		"arguments": args,
	}
	var lastErr error
	attempts := retries + 1
	for i := 0; i < attempts; i++ {
		attemptCtx := ctx
		cancel := func() {}
		if _, ok := ctx.Deadline(); !ok {
			attemptCtx, cancel = context.WithTimeout(ctx, 45*time.Second)
		}
		var result mcpCallResult
		err := c.call(attemptCtx, "tools/call", payload, &result)
		cancel()
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if i < retries {
				time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
				continue
			}
			return "", err
		}
		if result.IsError {
			lastErr = fmt.Errorf("tool call failed: %s", name)
			if i < retries {
				time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
				continue
			}
			return "", lastErr
		}
		var buf bytes.Buffer
		for _, item := range result.Content {
			if item.Type == "text" {
				buf.WriteString(item.Text)
			}
		}
		return buf.String(), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("tool call failed: %s", name)
	}
	return "", lastErr
}

func (c *MCPClient) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "bops",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	}
	var resp map[string]any
	if err := c.call(ctx, "initialize", params, &resp); err != nil {
		return err
	}
	_ = c.notify("initialized", nil)
	return nil
}

func (c *MCPClient) call(ctx context.Context, method string, params any, out any) error {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan responseResult, 1)
	if c.pending == nil {
		c.pending = make(map[int]chan responseResult)
	}
	c.pending[id] = ch
	c.mu.Unlock()

	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := c.send(req); err != nil {
		c.removePending(id)
		return err
	}
	resp, err := c.waitResponse(ctx, id, ch)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

func (c *MCPClient) notify(method string, params any) error {
	req := mcpRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.send(req)
}

func (c *MCPClient) send(req mcpRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = c.stdin.Write(payload)
	return err
}

type responseResult struct {
	resp *mcpResponse
	err  error
}

func (c *MCPClient) waitResponse(ctx context.Context, id int, ch chan responseResult) (*mcpResponse, error) {
	defer c.removePending(id)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("mcp response channel closed")
		}
		return result.resp, result.err
	}
}

func (c *MCPClient) readLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			c.failPending(err)
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var resp mcpResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.ID == 0 {
			continue
		}
		c.dispatchResponse(resp)
	}
}

func (c *MCPClient) dispatchResponse(resp mcpResponse) {
	c.mu.Lock()
	ch := c.pending[resp.ID]
	if ch != nil {
		delete(c.pending, resp.ID)
	}
	c.mu.Unlock()
	if ch == nil {
		return
	}
	ch <- responseResult{resp: &resp}
	close(ch)
}

func (c *MCPClient) removePending(id int) {
	c.mu.Lock()
	if c.pending != nil {
		delete(c.pending, id)
	}
	c.mu.Unlock()
}

func (c *MCPClient) failPending(err error) {
	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[int]chan responseResult)
	c.mu.Unlock()
	if pending == nil {
		return
	}
	for _, ch := range pending {
		ch <- responseResult{err: err}
		close(ch)
	}
}
