package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	sjsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const (
	DefaultSchemaPath = "docs/skills/skill.schema.json"
	ManifestFileName  = "skill.yaml"
)

type LoadedSkill struct {
	Manifest      Manifest
	SystemMessage *schema.Message
	Tools         []tool.InvokableTool
	SourceDir     string
	Clients       []*MCPClient
}

type Loader struct {
	Root        string
	SchemaPath  string
	Permissions PermissionChecker
	Audit       AuditSink

	once   sync.Once
	schema *sjsonschema.Schema
	err    error
}

func NewLoader(root string) *Loader {
	return &Loader{Root: root}
}

func (l *Loader) Load(skillName string) (*LoadedSkill, error) {
	name := strings.TrimSpace(skillName)
	if name == "" {
		return nil, NewLoadError("", "", "name", "skill name is required", "provide a non-empty skill name", nil)
	}

	root := l.Root
	if root == "" {
		root = DefaultRoot
	}

	skillDir := ResolveSkillDir(root, name)
	manifestPath := filepath.Join(skillDir, ManifestFileName)
	rawData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, NewLoadError(name, manifestPath, "skill.yaml", "failed to read skill manifest", "check skill.yaml path", err)
	}

	normalized, err := normalizeYAML(rawData)
	if err != nil {
		return nil, NewLoadError(name, manifestPath, "skill.yaml", "invalid YAML", "fix YAML syntax", err)
	}

	if err := l.validateManifest(normalized, manifestPath, name); err != nil {
		return nil, err
	}

	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, NewLoadError(name, manifestPath, "skill.yaml", "failed to marshal manifest", "check YAML structure", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(jsonBytes, &manifest); err != nil {
		return nil, NewLoadError(name, manifestPath, "skill.yaml", "failed to decode manifest", "check manifest fields", err)
	}

	memoryText, err := l.loadMemory(skillDir, name, manifest.Memory)
	if err != nil {
		return nil, err
	}

	systemPrompt := buildSystemPrompt(&manifest, memoryText)
	systemMessage := &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	}

	tools, clients, err := l.buildTools(skillDir, name, manifest)
	if err != nil {
		return nil, err
	}

	return &LoadedSkill{
		Manifest:      manifest,
		SystemMessage: systemMessage,
		Tools:         tools,
		SourceDir:     skillDir,
		Clients:       clients,
	}, nil
}

func (l *Loader) loadMemory(skillDir, skillName string, memory *Memory) (string, error) {
	if memory == nil {
		return "", nil
	}
	strategy := strings.TrimSpace(memory.Strategy)
	if strategy == "" {
		return "", nil
	}
	if strategy != "context" {
		return "", NewLoadError(skillName, skillDir, "memory.strategy", "unsupported memory strategy", "use strategy: context", nil)
	}

	var builder strings.Builder
	for _, file := range memory.Files {
		relative := strings.TrimSpace(file)
		if relative == "" {
			continue
		}
		path, err := resolveSkillFile(skillDir, relative)
		if err != nil {
			return "", NewLoadError(skillName, path, "memory.files", "memory file path is invalid", "use relative file paths under the skill directory", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", NewLoadError(skillName, path, "memory.files", "failed to read memory file", "ensure the file exists", err)
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		builder.WriteString("### ")
		builder.WriteString(relative)
		builder.WriteString("\n")
		builder.WriteString(content)
		builder.WriteString("\n\n")
	}

	return strings.TrimSpace(builder.String()), nil
}

func (l *Loader) buildTools(skillDir, skillName string, manifest Manifest) ([]tool.InvokableTool, []*MCPClient, error) {
	executables := manifest.Executables
	tools := make([]tool.InvokableTool, 0, len(executables))
	clients := make([]*MCPClient, 0)
	for _, exec := range executables {
		switch exec.Type {
		case "script", "binary":
			tool, err := NewExecTool(exec, skillDir, skillName, manifest.Permissions, l.Permissions, l.Audit)
			if err != nil {
				return nil, nil, NewLoadError(skillName, skillDir, "executables", "failed to build executable tool", "check executable definitions", err)
			}
			tools = append(tools, tool)
		case "mcp":
			if strings.TrimSpace(exec.Command) == "" {
				return nil, nil, NewLoadError(skillName, skillDir, "executables", "mcp command is required", "set executables.command", nil)
			}
			client, err := NewMCPClient(context.Background(), exec.Command, exec.Args, skillDir)
			if err != nil {
				return nil, nil, NewLoadError(skillName, skillDir, "executables", "failed to start mcp client", "check mcp command/args", err)
			}
			clients = append(clients, client)
			list, err := client.ListTools(context.Background())
			if err != nil {
				return nil, nil, NewLoadError(skillName, skillDir, "executables", "failed to list mcp tools", "check mcp server implementation", err)
			}
			for _, item := range list {
				mcpTool, err := NewMCPTool(client, item, skillName, manifest.Permissions, l.Permissions, l.Audit)
				if err != nil {
					return nil, nil, NewLoadError(skillName, skillDir, "executables", "failed to build mcp tool", "check mcp tool schema", err)
				}
				tools = append(tools, mcpTool)
			}
		default:
			return nil, nil, NewLoadError(skillName, skillDir, "executables", fmt.Sprintf("unsupported executable type: %s", exec.Type), "use script, binary, or mcp", nil)
		}
	}
	return tools, clients, nil
}

func (l *Loader) validateManifest(raw any, manifestPath, skillName string) error {
	schema, err := l.compileSchema()
	if err != nil {
		return NewLoadError(skillName, manifestPath, "schema", "failed to compile skill schema", "check schema path", err)
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return NewLoadError(skillName, manifestPath, "skill.yaml", "failed to marshal manifest", "check YAML syntax", err)
	}

	instance, err := sjsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		return NewLoadError(skillName, manifestPath, "skill.yaml", "invalid manifest JSON", "check YAML structure", err)
	}

	if err := schema.Validate(instance); err != nil {
		field := ""
		if vErr, ok := err.(*sjsonschema.ValidationError); ok {
			field = strings.Join(vErr.InstanceLocation, ".")
		}
		return NewLoadError(skillName, manifestPath, field, "skill.yaml validation failed", "fix missing/invalid fields", err)
	}
	return nil
}

func (l *Loader) compileSchema() (*sjsonschema.Schema, error) {
	l.once.Do(func() {
		schemaPath := l.SchemaPath
		if schemaPath == "" {
			schemaPath = DefaultSchemaPath
		}
		absPath, err := filepath.Abs(schemaPath)
		if err != nil {
			l.err = err
			return
		}
		compiler := sjsonschema.NewCompiler()
		compiled, err := compiler.Compile(absPath)
		if err != nil {
			l.err = err
			return
		}
		l.schema = compiled
	})
	return l.schema, l.err
}

func normalizeYAML(raw []byte) (any, error) {
	var payload any
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return normalizeYAMLValue(payload), nil
}

func normalizeYAMLValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, val := range v {
			out[key] = normalizeYAMLValue(val)
		}
		return out
	case map[any]any:
		out := map[string]any{}
		for key, val := range v {
			keyStr, ok := key.(string)
			if !ok {
				continue
			}
			out[keyStr] = normalizeYAMLValue(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeYAMLValue(item))
		}
		return out
	default:
		return v
	}
}

func buildSystemPrompt(manifest *Manifest, memoryText string) string {
	if manifest == nil {
		return ""
	}
	sections := []string{}
	if manifest.Profile.Role != "" {
		sections = append(sections, fmt.Sprintf("Role: %s", manifest.Profile.Role))
	}
	if instruction := strings.TrimSpace(manifest.Profile.Instruction); instruction != "" {
		sections = append(sections, instruction)
	}
	if memoryText != "" {
		sections = append(sections, "Memory:\n"+memoryText)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func resolveSkillFile(skillDir, relative string) (string, error) {
	if strings.TrimSpace(relative) == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("absolute path is not allowed")
	}
	path := filepath.Clean(filepath.Join(skillDir, relative))
	rel, err := filepath.Rel(skillDir, path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path escapes skill directory")
	}
	return path, nil
}
