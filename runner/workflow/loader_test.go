package workflow

import "testing"

const sampleWorkflowYAML = `version: v0.1
name: demo
description: mapping test
env_packages:
  - base

inventory:
  groups:
    web:
      hosts:
        - web1

plan:
  mode: auto
  strategy: sequential

steps:
  - name: run cmd
    targets: [web]
    action: cmd.run
    args:
      cmd: "echo hello"
      dir: "/tmp"

  - name: run shell
    targets: [web]
    action: script.shell
    args:
      script: |
        echo "hi"

  - name: set env
    targets: [web]
    action: env.set
    args:
      env:
        TOKEN: "abc"

  - name: install package
    targets: [web]
    action: pkg.install
    args:
      name: nginx

  - name: render template
    targets: [web]
    action: template.render
    args:
      src: nginx.conf.j2
      dest: /etc/nginx/nginx.conf

  - name: ensure service
    targets: [web]
    action: service.ensure
    args:
      name: nginx
      state: started
`

func TestLoadMapping(t *testing.T) {
	wf, err := Load([]byte(sampleWorkflowYAML))
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	if err := wf.Validate(); err != nil {
		t.Fatalf("validate workflow: %v", err)
	}

	if got := len(wf.Steps); got != 6 {
		t.Fatalf("expected 6 steps, got %d", got)
	}

	if got := wf.Steps[0].Args["cmd"]; got != "echo hello" {
		t.Fatalf("step0 cmd mismatch: %v", got)
	}
	if got := wf.Steps[0].Args["dir"]; got != "/tmp" {
		t.Fatalf("step0 dir mismatch: %v", got)
	}

	env, ok := wf.Steps[2].Args["env"].(map[string]any)
	if !ok {
		t.Fatalf("step2 env should be map")
	}
	if env["TOKEN"] != "abc" {
		t.Fatalf("step2 env token mismatch: %v", env["TOKEN"])
	}

	if got := wf.Steps[3].Args["name"]; got != "nginx" {
		t.Fatalf("step3 name mismatch: %v", got)
	}
	if got := wf.Steps[4].Args["src"]; got != "nginx.conf.j2" {
		t.Fatalf("step4 src mismatch: %v", got)
	}
	if got := wf.Steps[4].Args["dest"]; got != "/etc/nginx/nginx.conf" {
		t.Fatalf("step4 dest mismatch: %v", got)
	}
	if got := wf.Steps[5].Args["state"]; got != "started" {
		t.Fatalf("step5 state mismatch: %v", got)
	}
}
