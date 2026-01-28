package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"
	sjsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type ExecTool struct {
	info      schema.ToolInfo
	command   []string
	workDir   string
	validator *sjsonschema.Schema
	skillName string
	perms     []string
	checker   PermissionChecker
	audit     AuditSink
}

func NewExecTool(execDef Executable, skillDir, skillName string, permissions []string, checker PermissionChecker, audit AuditSink) (*ExecTool, error) {
	name := strings.TrimSpace(execDef.Name)
	if name == "" {
		return nil, fmt.Errorf("executable name is required")
	}
	execType := strings.TrimSpace(execDef.Type)
	if execType == "" {
		return nil, fmt.Errorf("executable type is required")
	}

	path, err := resolveExecutablePath(skillDir, execDef.Path, execType)
	if err != nil {
		return nil, err
	}

	command := buildCommand(execDef, path)
	if len(command) == 0 {
		return nil, fmt.Errorf("command is empty")
	}

	info := schema.ToolInfo{
		Name: name,
		Desc: strings.TrimSpace(execDef.Description),
	}

	var validator *sjsonschema.Schema
	if execDef.Parameters != nil {
		paramsSchema, compiled, err := parseParameterSchema(execDef.Parameters)
		if err != nil {
			return nil, err
		}
		info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(paramsSchema)
		validator = compiled
	}

	return &ExecTool{
		info:      info,
		command:   command,
		workDir:   skillDir,
		validator: validator,
		skillName: skillName,
		perms:     permissions,
		checker:   checker,
		audit:     audit,
	}, nil
}

func (t *ExecTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &t.info, nil
}

func (t *ExecTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if err := checkPermissions(t.skillName, t.info.Name, t.perms, t.checker, t.audit); err != nil {
		return "", err
	}
	payload := strings.TrimSpace(argumentsInJSON)
	if payload == "" {
		payload = "{}"
	}
	inst, err := sjsonschema.UnmarshalJSON(strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}
	if t.validator != nil {
		if err := t.validator.Validate(inst); err != nil {
			return "", fmt.Errorf("tool arguments validation failed: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, t.command[0], t.command[1:]...)
	cmd.Dir = t.workDir
	cmd.Stdin = strings.NewReader(payload)
	cmd.Env = buildToolEnv(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput != "" {
			return "", fmt.Errorf("tool execution failed: %s", errOutput)
		}
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return strings.TrimSpace(stderr.String()), nil
	}
	return output, nil
}

func buildCommand(execDef Executable, resolvedPath string) []string {
	args := append([]string{}, execDef.Args...)
	switch execDef.Type {
	case "script":
		runner := strings.TrimSpace(execDef.Runner)
		if runner == "" {
			return nil
		}
		return append([]string{runner, resolvedPath}, args...)
	case "binary":
		if resolvedPath == "" {
			return nil
		}
		return append([]string{resolvedPath}, args...)
	default:
		return nil
	}
}

func resolveExecutablePath(skillDir, path, execType string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		if execType == "script" {
			return "", fmt.Errorf("script path is required")
		}
		return "", nil
	}
	if filepath.IsAbs(trimmed) {
		if execType == "script" {
			return "", fmt.Errorf("script path must be relative to the skill directory")
		}
		return trimmed, nil
	}
	resolved := filepath.Clean(filepath.Join(skillDir, trimmed))
	return resolved, nil
}

func parseParameterSchema(raw any) (*einojsonschema.Schema, *sjsonschema.Schema, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode parameter schema: %w", err)
	}
	var einoSchema einojsonschema.Schema
	if err := json.Unmarshal(data, &einoSchema); err != nil {
		return nil, nil, fmt.Errorf("invalid parameter schema: %w", err)
	}

	compiler := sjsonschema.NewCompiler()
	resource, err := sjsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid parameter schema json: %w", err)
	}
	if err := compiler.AddResource("params.json", resource); err != nil {
		return nil, nil, fmt.Errorf("invalid parameter schema resource: %w", err)
	}
	compiled, err := compiler.Compile("params.json")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile parameter schema: %w", err)
	}

	return &einoSchema, compiled, nil
}

func buildToolEnv(argsJSON string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "BOPS_ARGS_JSON="+argsJSON)

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return env
	}
	for key, value := range args {
		name := "BOPS_ARG_" + sanitizeEnvKey(key)
		env = append(env, name+"="+stringifyValue(value))
	}
	return env
}

func sanitizeEnvKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return strings.ToUpper(b.String())
}

func stringifyValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}
