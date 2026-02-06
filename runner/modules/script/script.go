package script

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"bops/runner/modules"
	"bops/runner/scriptstore"
)

type Module struct {
	language string
	store    *scriptstore.Store
}

func New(language string, store *scriptstore.Store) *Module {
	return &Module{language: language, store: store}
}

func (m *Module) Check(ctx context.Context, req modules.Request) (modules.Result, error) {
	source, err := readScript(m.store, req, m.language)
	if err != nil {
		return modules.Result{}, err
	}
	return modules.Result{
		Changed: true,
		Diff: map[string]any{
			"language": m.language,
			"script":   source,
		},
	}, nil
}

func (m *Module) Apply(ctx context.Context, req modules.Request) (modules.Result, error) {
	script, err := readScript(m.store, req, m.language)
	if err != nil {
		return modules.Result{}, err
	}
	args, err := readArgs(req)
	if err != nil {
		return modules.Result{}, err
	}

	var execCmd *exec.Cmd
	switch m.language {
	case "shell":
		execCmd = exec.CommandContext(ctx, "/bin/sh", append([]string{"-s", "--"}, args...)...)
	case "python":
		execCmd = exec.CommandContext(ctx, "python3", append([]string{"-"}, args...)...)
	default:
		return modules.Result{}, fmt.Errorf("unsupported script language: %s", m.language)
	}

	if dir, ok := readString(req, "dir"); ok {
		execCmd.Dir = dir
	}
	if env, ok := readEnv(req); ok {
		execCmd.Env = append(os.Environ(), env...)
	}

	execCmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	stdoutWriter := io.Writer(&stdout)
	stderrWriter := io.Writer(&stderr)
	if req.Stdout != nil {
		stdoutWriter = io.MultiWriter(stdoutWriter, req.Stdout)
	}
	if req.Stderr != nil {
		stderrWriter = io.MultiWriter(stderrWriter, req.Stderr)
	}
	execCmd.Stdout = stdoutWriter
	execCmd.Stderr = stderrWriter

	err = execCmd.Run()
	stdoutText, stderrText := modules.ApplyOutputLimits(req, stdout.String(), stderr.String())
	result := modules.Result{
		Changed: true,
		Output: map[string]any{
			"stdout": stdoutText,
			"stderr": stderrText,
		},
	}
	if modules.ExportVarsEnabled(req) {
		if exports := modules.ParseExportVars(stdoutText); len(exports) > 0 {
			result.Output["vars"] = exports
		}
	}
	if err != nil {
		return result, fmt.Errorf("script.%s failed: %w", m.language, err)
	}
	return result, nil
}

func (m *Module) Rollback(ctx context.Context, req modules.Request) (modules.Result, error) {
	return modules.Result{}, fmt.Errorf("script.%s rollback not supported", m.language)
}

func readScript(store *scriptstore.Store, req modules.Request, expectedLanguage string) (string, error) {
	inline, inlineOK := readString(req, "script")
	ref, refOK := readString(req, "script_ref")
	if inlineOK && refOK {
		return "", fmt.Errorf("script and script_ref cannot be used together")
	}
	if !inlineOK && !refOK {
		return "", fmt.Errorf("script requires args.script or args.script_ref")
	}
	if inlineOK {
		if strings.TrimSpace(inline) == "" {
			return "", fmt.Errorf("script content is empty")
		}
		return inline, nil
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("script_ref is empty")
	}
	if store == nil {
		return "", fmt.Errorf("script store is not configured")
	}
	script, _, err := store.Get(ref)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(script.Content) == "" {
		return "", fmt.Errorf("script %q is empty", ref)
	}
	if script.Language != "" && expectedLanguage != "" && script.Language != expectedLanguage {
		return "", fmt.Errorf("script %q language mismatch", ref)
	}
	return script.Content, nil
}

func readArgs(req modules.Request) ([]string, error) {
	if req.Step.Args == nil {
		return nil, nil
	}
	raw, ok := req.Step.Args["args"]
	if !ok || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []string:
		return append([]string{}, v...), nil
	case []any:
		args := make([]string, 0, len(v))
		for _, item := range v {
			args = append(args, fmt.Sprint(item))
		}
		return args, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return []string{v}, nil
	default:
		return nil, fmt.Errorf("args must be list or string")
	}
}

func readString(req modules.Request, key string) (string, bool) {
	if req.Step.Args == nil {
		return "", false
	}
	val, ok := req.Step.Args[key]
	if !ok {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	default:
		return fmt.Sprint(v), true
	}
}

func readEnv(req modules.Request) ([]string, bool) {
	merged := map[string]string{}

	if req.Vars != nil {
		if raw, ok := req.Vars["env"]; ok {
			mergeEnvMap(merged, raw)
		}
	}
	if req.Step.Args != nil {
		if raw, ok := req.Step.Args["env"]; ok {
			mergeEnvMap(merged, raw)
		}
	}

	if len(merged) == 0 {
		return nil, false
	}

	result := make([]string, 0, len(merged))
	for k, v := range merged {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result, true
}

func mergeEnvMap(dst map[string]string, raw any) {
	switch env := raw.(type) {
	case map[string]any:
		for k, v := range env {
			dst[k] = fmt.Sprint(v)
		}
	case map[any]any:
		for k, v := range env {
			dst[fmt.Sprint(k)] = fmt.Sprint(v)
		}
	case map[string]string:
		for k, v := range env {
			dst[k] = v
		}
	}
}
