package skills

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type AgentSpec struct {
	Name   string
	Model  string
	Skills []string
}

type AgentBundle struct {
	Spec          AgentSpec
	SystemPrompt  string
	SystemMessage *schema.Message
	Tools         []tool.InvokableTool
	Skills        []RegisteredSkill
}

type ToolConflictPolicy string

const (
	ToolConflictError        ToolConflictPolicy = "error"
	ToolConflictOverwrite    ToolConflictPolicy = "overwrite"
	ToolConflictKeepExisting ToolConflictPolicy = "keep"
	ToolConflictPrefix       ToolConflictPolicy = "prefix"
)

type AgentFactory struct {
	registry *Registry
	policy   ToolConflictPolicy
}

type AgentFactoryOption func(*AgentFactory)

func WithToolConflictPolicy(policy ToolConflictPolicy) AgentFactoryOption {
	return func(f *AgentFactory) {
		if policy != "" {
			f.policy = policy
		}
	}
}

func NewAgentFactory(registry *Registry, opts ...AgentFactoryOption) *AgentFactory {
	f := &AgentFactory{
		registry: registry,
		policy:   ToolConflictError,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *AgentFactory) Build(spec AgentSpec) (*AgentBundle, error) {
	bundle, err := f.BuildWithContext(spec, TemplateContext{})
	if err != nil {
		return nil, err
	}
	if err := validateBundleIsolation(spec, bundle); err != nil {
		return nil, err
	}
	return bundle, nil
}

func (f *AgentFactory) BuildWithContext(spec AgentSpec, ctx TemplateContext) (*AgentBundle, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("agent name is required")
	}
	if len(spec.Skills) == 0 {
		return nil, fmt.Errorf("agent %q has no skills", name)
	}
	if f.registry == nil {
		return nil, fmt.Errorf("registry is not configured")
	}

	skills := make([]RegisteredSkill, 0, len(spec.Skills))
	tools := make([]tool.InvokableTool, 0)
	toolMap := make(map[string]tool.InvokableTool)
	promptParts := make([]string, 0, len(spec.Skills))

	for _, ref := range spec.Skills {
		skillName, version := splitSkillRef(ref)
		entry, ok := f.resolveSkill(skillName, version)
		if !ok {
			return nil, fmt.Errorf("skill not found: %s", ref)
		}
		if entry.Err != nil {
			return nil, fmt.Errorf("skill load failed: %s", ref)
		}
		if entry.Skill == nil {
			return nil, fmt.Errorf("skill is empty: %s", ref)
		}

		skills = append(skills, entry)
		if entry.Skill.SystemMessage != nil && strings.TrimSpace(entry.Skill.SystemMessage.Content) != "" {
			rendered, err := RenderTemplate(entry.Skill.SystemMessage.Content, buildTemplateData(ctx, spec, entry.Skill.Manifest))
			if err != nil {
				return nil, fmt.Errorf("render skill template failed: %s", entry.Name)
			}
			if strings.TrimSpace(rendered) != "" {
				promptParts = append(promptParts, buildSkillPromptHeader(entry, rendered))
			}
		}
		for _, t := range entry.Skill.Tools {
			info, err := t.Info(nil)
			if err != nil {
				return nil, fmt.Errorf("tool info failed for skill %s: %w", entry.Name, err)
			}
			key := info.Name
			if _, exists := toolMap[key]; exists {
				switch f.policy {
				case ToolConflictError:
					return nil, fmt.Errorf("tool conflict: %s", key)
				case ToolConflictKeepExisting:
					continue
				case ToolConflictOverwrite:
					toolMap[key] = t
				case ToolConflictPrefix:
					prefixed := fmt.Sprintf("%s__%s", entry.Name, key)
					if _, dup := toolMap[prefixed]; dup {
						return nil, fmt.Errorf("tool conflict: %s", prefixed)
					}
					toolMap[prefixed] = NewAliasTool(prefixed, t)
				default:
					return nil, fmt.Errorf("unknown tool conflict policy: %s", f.policy)
				}
			} else {
				toolMap[key] = t
			}
		}
	}

	for _, t := range toolMap {
		tools = append(tools, t)
	}

	systemPrompt := strings.TrimSpace(strings.Join(promptParts, "\n\n"))
	systemMessage := &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	}

	return &AgentBundle{
		Spec:          spec,
		SystemPrompt:  systemPrompt,
		SystemMessage: systemMessage,
		Tools:         tools,
		Skills:        skills,
	}, nil
}

func validateBundleIsolation(spec AgentSpec, bundle *AgentBundle) error {
	if bundle == nil {
		return nil
	}
	expected := make(map[string]struct{})
	for _, ref := range spec.Skills {
		name, _ := splitSkillRef(ref)
		if strings.TrimSpace(name) == "" {
			continue
		}
		expected[name] = struct{}{}
	}
	for _, skill := range bundle.Skills {
		if _, ok := expected[skill.Name]; !ok {
			return fmt.Errorf("tool bundle isolation failed: %s not requested", skill.Name)
		}
	}
	return nil
}

func (f *AgentFactory) resolveSkill(name, version string) (RegisteredSkill, bool) {
	if version != "" {
		return f.registry.Get(name, version)
	}
	if item, ok := f.registry.Get(name, ""); ok {
		return item, true
	}
	items := f.registry.FindByName(name)
	if len(items) == 1 {
		return items[0], true
	}
	return RegisteredSkill{}, false
}

func splitSkillRef(ref string) (string, string) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", ""
	}
	parts := strings.Split(trimmed, "@")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return trimmed, ""
}

func buildSkillPromptHeader(skill RegisteredSkill, content string) string {
	title := skill.Name
	if skill.Version != "" {
		title = fmt.Sprintf("%s@%s", skill.Name, skill.Version)
	}
	return strings.TrimSpace(fmt.Sprintf("## Skill: %s\n%s", title, content))
}
