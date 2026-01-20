package validationrun

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"bops/internal/validationenv"
)

type Result struct {
	Status string `json:"status"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
}

const remoteCommand = "cat > /tmp/bops-workflow.yaml && bops apply -f /tmp/bops-workflow.yaml"

func Run(ctx context.Context, env validationenv.ValidationEnv, yaml string) (Result, error) {
	switch env.Type {
	case validationenv.EnvTypeContainer:
		return runInContainer(ctx, env, yaml)
	case validationenv.EnvTypeSSH:
		return runOverSSH(ctx, env.Host, env.User, env.SSHKey, yaml)
	case validationenv.EnvTypeAgent:
		return runOverSSH(ctx, env.AgentAddress, env.User, env.SSHKey, yaml)
	default:
		return Result{}, fmt.Errorf("unsupported validation env type: %s", env.Type)
	}
}

var Runner = Run

func runInContainer(ctx context.Context, env validationenv.ValidationEnv, yaml string) (Result, error) {
	if strings.TrimSpace(env.Image) == "" {
		return Result{}, fmt.Errorf("container image is required")
	}
	args := []string{"run", "--rm", "-i", env.Image, "sh", "-c", remoteCommand}
	return runCommand(ctx, "docker", args, yaml)
}

func runOverSSH(ctx context.Context, host, user, keyPath, yaml string) (Result, error) {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return Result{}, fmt.Errorf("ssh host is required")
	}
	addr, port := splitHostPort(trimmed)
	if user != "" {
		addr = user + "@" + addr
	}
	args := []string{}
	if port != "" {
		args = append(args, "-p", port)
	}
	if strings.TrimSpace(keyPath) != "" {
		args = append(args, "-i", keyPath)
	}
	args = append(args, addr, "sh", "-c", remoteCommand)
	return runCommand(ctx, "ssh", args, yaml)
}

func runCommand(ctx context.Context, bin string, args []string, input string) (Result, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Status: "success",
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Code:   0,
	}
	if err != nil {
		result.Status = "failed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Code = exitErr.ExitCode()
		}
		return result, err
	}
	return result, nil
}

func splitHostPort(host string) (string, string) {
	parts := strings.Split(host, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return host, ""
}
