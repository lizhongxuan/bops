package envset

import (
	"context"
	"fmt"
	"strings"

	"bops/runner/modules"
)

type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Check(ctx context.Context, req modules.Request) (modules.Result, error) {
	env, err := readEnvMap(req)
	if err != nil {
		return modules.Result{}, err
	}
	return modules.Result{
		Changed: true,
		Diff: map[string]any{
			"env": env,
		},
		Output: map[string]any{
			"env": env,
		},
	}, nil
}

func (m *Module) Apply(ctx context.Context, req modules.Request) (modules.Result, error) {
	env, err := readEnvMap(req)
	if err != nil {
		return modules.Result{}, err
	}
	return modules.Result{
		Changed: true,
		Output: map[string]any{
			"env": env,
		},
	}, nil
}

func (m *Module) Rollback(ctx context.Context, req modules.Request) (modules.Result, error) {
	return modules.Result{}, fmt.Errorf("env.set rollback not supported")
}

func readEnvMap(req modules.Request) (map[string]string, error) {
	if req.Step.Args == nil {
		return nil, fmt.Errorf("env.set requires args.env")
	}
	raw, ok := req.Step.Args["env"]
	if !ok {
		return nil, fmt.Errorf("env.set requires args.env")
	}

	env := map[string]string{}
	switch v := raw.(type) {
	case map[string]any:
		for key, value := range v {
			env[key] = fmt.Sprint(value)
		}
	case map[any]any:
		for key, value := range v {
			env[fmt.Sprint(key)] = fmt.Sprint(value)
		}
	case map[string]string:
		for key, value := range v {
			env[key] = value
		}
	default:
		return nil, fmt.Errorf("env.set requires args.env to be a map")
	}

	if len(env) == 0 {
		return nil, fmt.Errorf("env.set requires at least one env entry")
	}

	for key := range env {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("env.set env key cannot be empty")
		}
	}

	return env, nil
}
