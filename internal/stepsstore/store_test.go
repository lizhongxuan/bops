package stepsstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStepsStorePutGet(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	stepsYAML := []byte(`version: v0.1
name: demo
description: sample
steps:
  - name: step1
    action: cmd.run
    args:
      cmd: "echo hi"
`)
	if _, err := store.PutSteps("demo", stepsYAML); err != nil {
		t.Fatalf("put steps: %v", err)
	}

	doc, raw, err := store.GetSteps("demo")
	if err != nil {
		t.Fatalf("get steps: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("expected raw steps yaml")
	}
	if doc.Name != "demo" {
		t.Fatalf("expected name demo, got %q", doc.Name)
	}
	if doc.Description != "sample" {
		t.Fatalf("expected description sample, got %q", doc.Description)
	}
	if len(doc.Steps) != 1 || doc.Steps[0].Name != "step1" {
		t.Fatalf("expected one step")
	}

	invYAML := []byte(`inventory:
  hosts:
    local:
      address: "127.0.0.1"
`)
	if _, err := store.PutInventory("demo", invYAML); err != nil {
		t.Fatalf("put inventory: %v", err)
	}

	invDoc, invRaw, err := store.GetInventory("demo")
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}
	if len(invRaw) == 0 {
		t.Fatalf("expected raw inventory yaml")
	}
	if len(invDoc.Inventory.Hosts) != 1 {
		t.Fatalf("expected one host in inventory")
	}
}

func TestLoadWorkflowDefaults(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	stepsYAML := []byte(`steps:
  - name: step1
    action: cmd.run
    args:
      cmd: "echo hi"
`)
	if _, err := store.PutSteps("demo", stepsYAML); err != nil {
		t.Fatalf("put steps: %v", err)
	}
	wf, err := store.LoadWorkflow("demo")
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	if wf.Name != "demo" {
		t.Fatalf("expected workflow name demo, got %q", wf.Name)
	}
	if wf.Version == "" {
		t.Fatalf("expected default version")
	}
	if wf.Plan.Mode == "" || wf.Plan.Strategy == "" {
		t.Fatalf("expected default plan")
	}
	if len(wf.Steps) != 1 {
		t.Fatalf("expected steps")
	}
	if len(wf.Inventory.Hosts) != 0 {
		t.Fatalf("expected empty inventory by default")
	}
}

func TestListUsesSteps(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	stepsYAML := []byte(`name: demo
description: hello
steps:
  - name: step1
    action: cmd.run
    args:
      cmd: "echo hi"
`)
	if _, err := store.PutSteps("demo", stepsYAML); err != nil {
		t.Fatalf("put steps: %v", err)
	}
	items, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item")
	}
	if items[0].Name != "demo" {
		t.Fatalf("expected demo, got %q", items[0].Name)
	}
	if items[0].Description != "hello" {
		t.Fatalf("expected description hello, got %q", items[0].Description)
	}
}

func TestPutStepsNameMismatch(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	stepsYAML := []byte(`name: other
steps:
  - name: step1
    action: cmd.run
    args:
      cmd: "echo hi"
`)
	if _, err := store.PutSteps("demo", stepsYAML); err == nil || !strings.Contains(err.Error(), "workflow name mismatch") {
		t.Fatalf("expected name mismatch error, got %v", err)
	}
}

func TestInventoryPath(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	invYAML := []byte(`inventory: {}`)
	if _, err := store.PutInventory("demo", invYAML); err != nil {
		t.Fatalf("put inventory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo", "inventory.yaml")); err != nil {
		t.Fatalf("expected inventory.yaml, got %v", err)
	}
}

func TestLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	legacy := []byte(`version: v0.1
name: demo
description: legacy
inventory:
  hosts:
    local:
      address: "127.0.0.1"
steps:
  - name: step1
    action: cmd.run
    args:
      cmd: "echo hi"
`)
	legacyPath := filepath.Join(dir, "demo.yaml")
	if err := os.WriteFile(legacyPath, legacy, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	doc, _, err := store.GetSteps("demo")
	if err != nil {
		t.Fatalf("get steps: %v", err)
	}
	if doc.Name != "demo" {
		t.Fatalf("expected demo after migration")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy file removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo", "steps.yaml")); err != nil {
		t.Fatalf("expected steps.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo", "inventory.yaml")); err != nil {
		t.Fatalf("expected inventory.yaml: %v", err)
	}
}
