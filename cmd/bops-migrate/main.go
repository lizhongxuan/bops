package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"bops/internal/stepsstore"
)

func main() {
	dataDir := flag.String("data", "data", "data directory")
	flag.Parse()

	store := stepsstore.New(filepath.Join(*dataDir, "workflows"))
	items, err := store.List()
	if err != nil {
		fmt.Printf("migration failed: %v\n", err)
		return
	}
	fmt.Printf("migration done, workflows=%d\n", len(items))
}
