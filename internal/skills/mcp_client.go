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
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	nextID int
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
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		nextID: 1,
	}
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
	payload := map[string]any{
		"name":      name,
		"arguments": args,
	}
	var result mcpCallResult
	if err := c.call(ctx, "tools/call", payload, &result); err != nil {
		return "", err
	}
	if result.IsError {
		return "", fmt.Errorf("tool call failed: %s", name)
	}
	var buf bytes.Buffer
	for _, item := range result.Content {
		if item.Type == "text" {
			buf.WriteString(item.Text)
		}
	}
	return buf.String(), nil
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
	c.mu.Unlock()

	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := c.send(req); err != nil {
		return err
	}
	resp, err := c.readResponse(ctx, id)
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

func (c *MCPClient) readResponse(ctx context.Context, id int) (*mcpResponse, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			return nil, err
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
		if resp.ID == id {
			return &resp, nil
		}
	}
}
