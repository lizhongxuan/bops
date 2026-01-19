package host

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

type RunOptions struct {
	Dir string
	Env []string
}

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Adapter interface {
	Run(ctx context.Context, cmd string, args []string, opts RunOptions) (RunResult, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	LookPath(file string) (string, error)
}

type LocalAdapter struct{}

func NewLocalAdapter() *LocalAdapter {
	return &LocalAdapter{}
}

func (a *LocalAdapter) Run(ctx context.Context, cmd string, args []string, opts RunOptions) (RunResult, error) {
	command := exec.CommandContext(ctx, cmd, args...)
	if opts.Dir != "" {
		command.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		command.Env = append(os.Environ(), opts.Env...)
	}

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}

func (a *LocalAdapter) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (a *LocalAdapter) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (a *LocalAdapter) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (a *LocalAdapter) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
