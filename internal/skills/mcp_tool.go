package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bops/internal/logging"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"
	sjsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"go.uber.org/zap"
)

type MCPTool struct {
	client    *MCPClient
	name      string
	info      schema.ToolInfo
	validator *sjsonschema.Schema
	skillName string
	perms     []string
	checker   PermissionChecker
	audit     AuditSink
}

func NewMCPTool(client *MCPClient, toolDef MCPToolDefinition, skillName string, permissions []string, checker PermissionChecker, audit AuditSink) (*MCPTool, error) {
	if client == nil {
		return nil, fmt.Errorf("mcp client is required")
	}
	name := strings.TrimSpace(toolDef.Name)
	if name == "" {
		return nil, fmt.Errorf("mcp tool name is required")
	}
	info := schema.ToolInfo{
		Name: name,
		Desc: strings.TrimSpace(toolDef.Description),
	}

	var validator *sjsonschema.Schema
	if len(toolDef.InputSchema) > 0 {
		var schemaDoc einojsonschema.Schema
		if err := json.Unmarshal(toolDef.InputSchema, &schemaDoc); err != nil {
			return nil, fmt.Errorf("invalid tool schema: %w", err)
		}
		info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&schemaDoc)

		compiled, err := compileRawSchema(toolDef.InputSchema)
		if err != nil {
			return nil, err
		}
		validator = compiled
	}

	return &MCPTool{
		client:    client,
		name:      name,
		info:      info,
		validator: validator,
		skillName: skillName,
		perms:     permissions,
		checker:   checker,
		audit:     audit,
	}, nil
}

func (t *MCPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &t.info, nil
}

func (t *MCPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if err := checkPermissions(t.skillName, t.name, t.perms, t.checker, t.audit); err != nil {
		return "", err
	}
	payload := strings.TrimSpace(argumentsInJSON)
	if payload == "" {
		payload = "{}"
	}
	started := time.Now()
	logging.L().Info("skill start",
		zap.String("skill", t.skillName),
		zap.String("tool", t.name),
		zap.String("args", payload),
	)
	inst, err := sjsonschema.UnmarshalJSON(strings.NewReader(payload))
	if err != nil {
		logging.L().Error("skill end",
			zap.String("skill", t.skillName),
			zap.String("tool", t.name),
			zap.Error(err),
			zap.Duration("elapsed", time.Since(started)),
		)
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}
	if t.validator != nil {
		if err := t.validator.Validate(inst); err != nil {
			logging.L().Error("skill end",
				zap.String("skill", t.skillName),
				zap.String("tool", t.name),
				zap.Error(err),
				zap.Duration("elapsed", time.Since(started)),
			)
			return "", fmt.Errorf("tool arguments validation failed: %w", err)
		}
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(payload), &args); err != nil {
		logging.L().Error("skill end",
			zap.String("skill", t.skillName),
			zap.String("tool", t.name),
			zap.Error(err),
			zap.Duration("elapsed", time.Since(started)),
		)
		return "", fmt.Errorf("invalid tool arguments json: %w", err)
	}
	output, err := t.client.CallTool(ctx, t.name, args)
	if err != nil {
		logging.L().Error("skill end",
			zap.String("skill", t.skillName),
			zap.String("tool", t.name),
			zap.Error(err),
			zap.Duration("elapsed", time.Since(started)),
		)
		return "", err
	}
	logging.L().Info("skill end",
		zap.String("skill", t.skillName),
		zap.String("tool", t.name),
		zap.Int("output_len", len(strings.TrimSpace(output))),
		zap.Duration("elapsed", time.Since(started)),
	)
	return output, nil
}

func compileRawSchema(raw []byte) (*sjsonschema.Schema, error) {
	compiler := sjsonschema.NewCompiler()
	resource, err := sjsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("invalid tool schema json: %w", err)
	}
	if err := compiler.AddResource("mcp-tool-schema.json", resource); err != nil {
		return nil, fmt.Errorf("invalid tool schema resource: %w", err)
	}
	compiled, err := compiler.Compile("mcp-tool-schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile tool schema: %w", err)
	}
	return compiled, nil
}
