package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"bops/runner/engine"
	"bops/runner/scriptstore"
	"bops/runner/workflow"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: runner-plan <yaml-file>")
		os.Exit(2)
	}

	yamlPath := os.Args[1]
	wf, err := workflow.LoadFile(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load workflow: %v\n", err)
		os.Exit(1)
	}
	if err := wf.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "validate workflow: %v\n", err)
		os.Exit(1)
	}

	store := scriptstore.New("./scripts")
	reg := engine.DefaultRegistry(store)
	eng := engine.New(reg)

	plan, err := eng.Plan(context.Background(), wf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan workflow: %v\n", err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(plan, "", "  ")
	fmt.Println(string(out))
}
