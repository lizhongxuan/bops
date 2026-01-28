package skills

import (
	"bytes"
	"strings"
	"text/template"
)

type TemplateContext struct {
	EnvPackages    []string
	Scripts        []string
	ValidationEnvs []string
	User           map[string]any
	Extra          map[string]any
}

func RenderTemplate(input string, data map[string]any) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", nil
	}
	tmpl, err := template.New("skill-profile").Option("missingkey=zero").Parse(input)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func buildTemplateData(ctx TemplateContext, agent AgentSpec, skill Manifest) map[string]any {
	data := map[string]any{
		"EnvPackages":    append([]string{}, ctx.EnvPackages...),
		"Scripts":        append([]string{}, ctx.Scripts...),
		"ValidationEnvs": append([]string{}, ctx.ValidationEnvs...),
		"User":           ctx.User,
		"Agent": map[string]any{
			"Name":   agent.Name,
			"Model":  agent.Model,
			"Skills": append([]string{}, agent.Skills...),
		},
		"Skill": map[string]any{
			"Name":        skill.Name,
			"Version":     skill.Version,
			"Description": skill.Description,
		},
	}
	if ctx.Extra != nil {
		data["Extra"] = ctx.Extra
	}
	return data
}
