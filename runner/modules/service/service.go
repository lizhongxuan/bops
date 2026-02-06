package service

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"bops/runner/modules"
)

type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Check(ctx context.Context, req modules.Request) (modules.Result, error) {
	name, err := readServiceName(req)
	if err != nil {
		return modules.Result{}, err
	}

	action := strings.TrimSpace(req.Step.Action)
	mgr, err := detectManager()
	if err != nil {
		return modules.Result{}, err
	}

	switch action {
	case "service.restart":
		return modules.Result{Changed: true}, nil
	case "service.ensure":
		desired := readDesiredState(req)
		active, err := mgr.isActive(ctx, name)
		if err != nil {
			return modules.Result{}, err
		}
		shouldChange := (desired == "started" && !active) || (desired == "stopped" && active)
		return modules.Result{Changed: shouldChange}, nil
	default:
		return modules.Result{}, fmt.Errorf("unsupported service action %q", action)
	}
}

func (m *Module) Apply(ctx context.Context, req modules.Request) (modules.Result, error) {
	name, err := readServiceName(req)
	if err != nil {
		return modules.Result{}, err
	}

	action := strings.TrimSpace(req.Step.Action)
	mgr, err := detectManager()
	if err != nil {
		return modules.Result{}, err
	}

	var stdout string
	var stderr string
	switch action {
	case "service.restart":
		stdout, stderr, err = mgr.restart(ctx, name)
	case "service.ensure":
		desired := readDesiredState(req)
		if desired == "stopped" {
			stdout, stderr, err = mgr.stop(ctx, name)
		} else {
			stdout, stderr, err = mgr.start(ctx, name)
		}
	default:
		return modules.Result{}, fmt.Errorf("unsupported service action %q", action)
	}

	result := modules.Result{
		Changed: true,
		Output: map[string]any{
			"stdout": stdout,
			"stderr": stderr,
		},
	}
	if err != nil {
		return result, fmt.Errorf("service action failed: %w", err)
	}
	return result, nil
}

func (m *Module) Rollback(ctx context.Context, req modules.Request) (modules.Result, error) {
	return modules.Result{}, fmt.Errorf("service rollback not supported")
}

type manager struct {
	name string
}

func detectManager() (manager, error) {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return manager{name: "systemctl"}, nil
	}
	if _, err := exec.LookPath("service"); err == nil {
		return manager{name: "service"}, nil
	}
	if _, err := exec.LookPath("rc-service"); err == nil {
		return manager{name: "rc-service"}, nil
	}
	return manager{}, fmt.Errorf("no supported service manager found")
}

func (m manager) isActive(ctx context.Context, name string) (bool, error) {
	var cmd *exec.Cmd
	switch m.name {
	case "systemctl":
		cmd = exec.CommandContext(ctx, "systemctl", "is-active", name)
	case "service":
		cmd = exec.CommandContext(ctx, "service", name, "status")
	case "rc-service":
		cmd = exec.CommandContext(ctx, "rc-service", name, "status")
	default:
		return false, fmt.Errorf("unsupported service manager")
	}
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m manager) start(ctx context.Context, name string) (string, string, error) {
	return m.run(ctx, name, "start")
}

func (m manager) stop(ctx context.Context, name string) (string, string, error) {
	return m.run(ctx, name, "stop")
}

func (m manager) restart(ctx context.Context, name string) (string, string, error) {
	return m.run(ctx, name, "restart")
}

func (m manager) run(ctx context.Context, name, action string) (string, string, error) {
	var cmd *exec.Cmd
	switch m.name {
	case "systemctl":
		cmd = exec.CommandContext(ctx, "systemctl", action, name)
	case "service":
		cmd = exec.CommandContext(ctx, "service", name, action)
	case "rc-service":
		cmd = exec.CommandContext(ctx, "rc-service", name, action)
	default:
		return "", "", fmt.Errorf("unsupported service manager")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func readServiceName(req modules.Request) (string, error) {
	if req.Step.Args == nil {
		return "", fmt.Errorf("service action requires args.name")
	}
	name, ok := req.Step.Args["name"]
	if !ok || strings.TrimSpace(fmt.Sprint(name)) == "" {
		return "", fmt.Errorf("service action requires args.name")
	}
	return fmt.Sprint(name), nil
}

func readDesiredState(req modules.Request) string {
	if req.Step.Args == nil {
		return "started"
	}
	state, ok := req.Step.Args["state"]
	if !ok {
		return "started"
	}
	value := strings.ToLower(strings.TrimSpace(fmt.Sprint(state)))
	if value == "stopped" {
		return "stopped"
	}
	return "started"
}
