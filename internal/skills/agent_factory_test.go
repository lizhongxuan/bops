package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentFactoryToolIsolation(t *testing.T) {
	root := t.TempDir()
	skillA := filepath.Join(root, "alpha-skill")
	skillB := filepath.Join(root, "beta-skill")

	writeSkill(t, skillA, "alpha-skill", "alpha", "scripts/alpha.sh")
	writeSkill(t, skillB, "beta-skill", "beta", "scripts/beta.sh")

	loader := NewLoader(root)
	loader.SchemaPath = schemaPathForAgent(t)
	registry := NewRegistry(loader)
	registry.Refresh([]string{"alpha-skill", "beta-skill"})

	factory := NewAgentFactory(registry)
	bundle, err := factory.Build(AgentSpec{
		Name:   "agent-a",
		Skills: []string{"alpha-skill"},
	})
	if err != nil {
		t.Fatalf("build agent bundle: %v", err)
	}

	toolNames := map[string]struct{}{}
	for _, toolItem := range bundle.Tools {
		info, err := toolItem.Info(nil)
		if err != nil {
			t.Fatalf("tool info: %v", err)
		}
		toolNames[info.Name] = struct{}{}
	}
	if _, ok := toolNames["alpha"]; !ok {
		t.Fatalf("expected alpha tool, got %v", toolNames)
	}
	if _, ok := toolNames["beta"]; ok {
		t.Fatalf("unexpected beta tool in isolated bundle")
	}
}

func writeSkill(t *testing.T, dir, name, toolName, toolPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(toolPath)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, toolPath), []byte("echo ok"), 0o644); err != nil {
		t.Fatalf("write tool: %v", err)
	}
	manifest := `
name: "` + name + `"
version: "1.0.0"
description: "test skill"
profile:
  role: "tester"
  instruction: "do tests"
executables:
  - name: "` + toolName + `"
    type: "script"
    runner: "sh"
    path: "` + toolPath + `"
`
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func schemaPathForAgent(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(filepath.Clean(filepath.Join(root, "..", "..")), "docs", "skills", "skill.schema.json")
}
