package tester

import (
	"context"
	"fmt"
	"os/exec"
)

type Result struct {
	Stdout string
	Stderr string
	Code   int
}

type DockerRunner struct {
	Binary string
}

func NewDockerRunner() *DockerRunner {
	return &DockerRunner{Binary: "docker"}
}

func (d *DockerRunner) StartAgent(ctx context.Context, image string, args []string) (Result, error) {
	if image == "" {
		return Result{}, fmt.Errorf("docker image is required")
	}

	cmdArgs := append([]string{"run", "--rm", image}, args...)
	cmd := exec.CommandContext(ctx, d.Binary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	result := Result{
		Stdout: string(output),
		Code:   0,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Code = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
		}
		return result, err
	}
	return result, nil
}
