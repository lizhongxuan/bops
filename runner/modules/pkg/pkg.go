package pkg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"bops/runner/modules"
)

type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Check(ctx context.Context, req modules.Request) (modules.Result, error) {
	packages, err := readPackages(req)
	if err != nil {
		return modules.Result{}, err
	}

	mgr, err := detectManager()
	if err != nil {
		return modules.Result{Changed: true}, nil
	}

	missing := []string{}
	for _, name := range packages {
		ok, err := mgr.isInstalled(ctx, name)
		if err != nil {
			return modules.Result{}, err
		}
		if !ok {
			missing = append(missing, name)
		}
	}

	return modules.Result{
		Changed: len(missing) > 0,
		Diff: map[string]any{
			"missing": missing,
		},
	}, nil
}

func (m *Module) Apply(ctx context.Context, req modules.Request) (modules.Result, error) {
	packages, err := readPackages(req)
	if err != nil {
		return modules.Result{}, err
	}

	mgr, err := detectManager()
	if err != nil {
		return modules.Result{}, err
	}

	stdout, stderr, err := mgr.install(ctx, packages)
	result := modules.Result{
		Changed: true,
		Output: map[string]any{
			"stdout": stdout,
			"stderr": stderr,
		},
	}
	if err != nil {
		return result, fmt.Errorf("pkg.install failed: %w", err)
	}
	return result, nil
}

func (m *Module) Rollback(ctx context.Context, req modules.Request) (modules.Result, error) {
	return modules.Result{}, fmt.Errorf("pkg.install rollback not supported")
}

type manager struct {
	name       string
	checkCmd   []string
	installCmd []string
}

func detectManager() (manager, error) {
	if path, err := exec.LookPath("apt-get"); err == nil {
		_ = path
		return manager{
			name:       "apt",
			checkCmd:   []string{"dpkg", "-s"},
			installCmd: []string{"apt-get", "install", "-y"},
		}, nil
	}
	if path, err := exec.LookPath("dnf"); err == nil {
		_ = path
		return manager{
			name:       "dnf",
			checkCmd:   []string{"rpm", "-q"},
			installCmd: []string{"dnf", "install", "-y"},
		}, nil
	}
	if path, err := exec.LookPath("yum"); err == nil {
		_ = path
		return manager{
			name:       "yum",
			checkCmd:   []string{"rpm", "-q"},
			installCmd: []string{"yum", "install", "-y"},
		}, nil
	}
	if path, err := exec.LookPath("apk"); err == nil {
		_ = path
		return manager{
			name:       "apk",
			checkCmd:   []string{"apk", "info", "-e"},
			installCmd: []string{"apk", "add", "--no-cache"},
		}, nil
	}
	if path, err := exec.LookPath("pacman"); err == nil {
		_ = path
		return manager{
			name:       "pacman",
			checkCmd:   []string{"pacman", "-Qi"},
			installCmd: []string{"pacman", "-S", "--noconfirm"},
		}, nil
	}
	return manager{}, fmt.Errorf("no supported package manager found")
}

func (m manager) isInstalled(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, m.checkCmd[0], append(m.checkCmd[1:], name)...)
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m manager) install(ctx context.Context, packages []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, m.installCmd[0], append(m.installCmd[1:], packages...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func readPackages(req modules.Request) ([]string, error) {
	if req.Step.Args == nil {
		return nil, fmt.Errorf("pkg.install requires args.name or args.names")
	}

	if name, ok := req.Step.Args["name"]; ok {
		return []string{fmt.Sprint(name)}, nil
	}
	if raw, ok := req.Step.Args["names"]; ok {
		switch names := raw.(type) {
		case []any:
			result := make([]string, 0, len(names))
			for _, item := range names {
				result = append(result, fmt.Sprint(item))
			}
			return result, nil
		case []string:
			return names, nil
		}
	}

	return nil, fmt.Errorf("pkg.install requires args.name or args.names")
}

func (m manager) String() string {
	return m.name
}
