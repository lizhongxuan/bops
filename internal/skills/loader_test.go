package skills

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/tool"
)

func TestLoadSkillValidation(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "bad-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	manifest := `
version: "0.1.0"
description: "bad skill"
profile:
  role: "tester"
  instruction: "do nothing"
executables:
  - name: "noop"
    type: "script"
    runner: "sh"
    path: "scripts/noop.sh"
`
	writeFile(t, filepath.Join(skillDir, "skill.yaml"), manifest)

	loader := NewLoader(root)
	loader.SchemaPath = schemaPath(t)

	_, err := loader.Load("bad-skill")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if !strings.Contains(loadErr.Message, "validation") {
		t.Fatalf("expected validation error message, got %q", loadErr.Message)
	}
}

func TestLoadSkillMemory(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo")
	writeFile(t, filepath.Join(skillDir, "knowledge", "guide.md"), "Guide content")
	writeFile(t, filepath.Join(skillDir, "knowledge", "notes.txt"), "Notes content")
	writeFile(t, filepath.Join(skillDir, "scripts", "echo.sh"), "echo ok")
	manifest := `
name: "demo"
version: "1.0.0"
description: "memory test"
profile:
  role: "tester"
  instruction: "use memory"
memory:
  strategy: "context"
  files:
    - "knowledge/guide.md"
    - "knowledge/notes.txt"
executables:
  - name: "echo"
    type: "script"
    runner: "sh"
    path: "scripts/echo.sh"
`
	writeFile(t, filepath.Join(skillDir, "skill.yaml"), manifest)

	loader := NewLoader(root)
	loader.SchemaPath = schemaPath(t)

	loaded, err := loader.Load("demo")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.SystemMessage == nil {
		t.Fatal("expected system message")
	}
	content := loaded.SystemMessage.Content
	if !strings.Contains(content, "Memory:") {
		t.Fatalf("expected memory section, got %q", content)
	}
	if !strings.Contains(content, "### knowledge/guide.md") || !strings.Contains(content, "Guide content") {
		t.Fatalf("expected guide content, got %q", content)
	}
	if !strings.Contains(content, "### knowledge/notes.txt") || !strings.Contains(content, "Notes content") {
		t.Fatalf("expected notes content, got %q", content)
	}
}

func TestBuildToolEnvInjection(t *testing.T) {
	args := `{"host":"example.com","count":2,"flags":["a","b"],"meta":{"ok":true}}`
	env := buildToolEnv(args)
	envMap := map[string]string{}
	for _, item := range env {
		if !strings.HasPrefix(item, "BOPS_") {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	if envMap["BOPS_ARGS_JSON"] != args {
		t.Fatalf("unexpected args json: %q", envMap["BOPS_ARGS_JSON"])
	}
	if envMap["BOPS_ARG_HOST"] != "example.com" {
		t.Fatalf("unexpected host: %q", envMap["BOPS_ARG_HOST"])
	}
	if envMap["BOPS_ARG_COUNT"] != "2" {
		t.Fatalf("unexpected count: %q", envMap["BOPS_ARG_COUNT"])
	}
	if envMap["BOPS_ARG_FLAGS"] != `["a","b"]` {
		t.Fatalf("unexpected flags: %q", envMap["BOPS_ARG_FLAGS"])
	}
	if envMap["BOPS_ARG_META"] != `{"ok":true}` {
		t.Fatalf("unexpected meta: %q", envMap["BOPS_ARG_META"])
	}
}

func TestDemoPingSkillIntegration(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	root := repoRoot(t)
	loader := NewLoader(filepath.Join(root, "skills"))
	loader.SchemaPath = filepath.Join(root, "docs", "skills", "skill.schema.json")

	loaded, err := loader.Load("demo-ping")
	if err != nil {
		t.Fatalf("load demo-ping failed: %v", err)
	}
	var found tool.InvokableTool
	for _, item := range loaded.Tools {
		info, err := item.Info(nil)
		if err != nil {
			t.Fatalf("tool info failed: %v", err)
		}
		if info.Name == "ping_host" {
			found = item
			break
		}
	}
	if found == nil {
		t.Fatal("ping_host tool not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := found.InvokableRun(ctx, `{"host":"127.0.0.1","count":2}`)
	if err != nil {
		t.Fatalf("tool run failed: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if payload["host"] != "127.0.0.1" {
		t.Fatalf("unexpected host output: %v", payload["host"])
	}
	if payload["count"] == nil {
		t.Fatalf("missing count output")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func schemaPath(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	return filepath.Join(root, "docs", "skills", "skill.schema.json")
}
