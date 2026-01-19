package template

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"bops/internal/modules"
)

type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Check(ctx context.Context, req modules.Request) (modules.Result, error) {
	rendered, dest, err := renderTemplate(req)
	if err != nil {
		return modules.Result{}, err
	}

	current, err := os.ReadFile(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return modules.Result{Changed: true}, nil
		}
		return modules.Result{}, err
	}

	changed := !bytes.Equal(current, rendered)
	return modules.Result{
		Changed: changed,
		Diff: map[string]any{
			"dest": dest,
		},
	}, nil
}

func (m *Module) Apply(ctx context.Context, req modules.Request) (modules.Result, error) {
	rendered, dest, err := renderTemplate(req)
	if err != nil {
		return modules.Result{}, err
	}

	mode := os.FileMode(0o644)
	if rawMode, ok := readMode(req); ok {
		mode = rawMode
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return modules.Result{}, err
	}

	if err := os.WriteFile(dest, rendered, mode); err != nil {
		return modules.Result{}, err
	}

	return modules.Result{
		Changed: true,
		Output: map[string]any{
			"dest": dest,
		},
	}, nil
}

func (m *Module) Rollback(ctx context.Context, req modules.Request) (modules.Result, error) {
	return modules.Result{}, fmt.Errorf("template.render rollback not supported")
}

func renderTemplate(req modules.Request) ([]byte, string, error) {
	if req.Step.With == nil {
		return nil, "", fmt.Errorf("template.render requires with.src and with.dest")
	}

	src, ok := readString(req, "src")
	if !ok || src == "" {
		return nil, "", fmt.Errorf("template.render requires with.src")
	}
	dest, ok := readString(req, "dest")
	if !ok || dest == "" {
		return nil, "", fmt.Errorf("template.render requires with.dest")
	}

	content, err := os.ReadFile(src)
	if err != nil {
		return nil, "", err
	}

	tpl, err := template.New(filepath.Base(src)).Parse(string(content))
	if err != nil {
		return nil, "", err
	}

	vars := mergeVars(req.Vars, readVars(req))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, vars); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), dest, nil
}

func readString(req modules.Request, key string) (string, bool) {
	val, ok := req.Step.With[key]
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

func readVars(req modules.Request) map[string]any {
	if req.Step.With == nil {
		return map[string]any{}
	}
	val, ok := req.Step.With["vars"]
	if !ok {
		return map[string]any{}
	}
	switch v := val.(type) {
	case map[string]any:
		return v
	case map[any]any:
		out := make(map[string]any, len(v))
		for k, value := range v {
			out[fmt.Sprint(k)] = value
		}
		return out
	default:
		return map[string]any{}
	}
}

func readMode(req modules.Request) (os.FileMode, bool) {
	if req.Step.With == nil {
		return 0, false
	}
	val, ok := req.Step.With["mode"]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return os.FileMode(v), true
	case int64:
		return os.FileMode(v), true
	case float64:
		return os.FileMode(int64(v)), true
	case string:
		parsed, err := strconv.ParseUint(v, 0, 32)
		if err != nil {
			return 0, false
		}
		return os.FileMode(parsed), true
	default:
		return 0, false
	}
}

func mergeVars(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
