package aiworkflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLoopMemoryStoreRoundTrip(t *testing.T) {
	store := NewFileLoopMemoryStore(t.TempDir())
	snapshot := defaultLoopMemorySnapshot("session-a")
	snapshot.PRD.UserStories = []LoopPRDStory{
		{ID: "US-1", Passes: true},
	}
	snapshot.Progress = "line1\nline2\n"
	snapshot.Checkpoint.Iteration = 3
	snapshot.Checkpoint.ToolHistory = []string{"tool=test output=PASS"}

	if err := store.Save(context.Background(), "session-a", snapshot); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, found, err := store.Load(context.Background(), "session-a")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !found {
		t.Fatalf("expected snapshot found")
	}
	if loaded.Checkpoint.Iteration != 3 {
		t.Fatalf("expected iteration 3, got %d", loaded.Checkpoint.Iteration)
	}
	if len(loaded.PRD.UserStories) != 1 || !loaded.PRD.UserStories[0].Passes {
		t.Fatalf("expected persisted PRD stories")
	}
	if !strings.Contains(loaded.Progress, "line2") {
		t.Fatalf("expected persisted progress")
	}
}

func TestFileLoopMemoryStoreCorruptedArtifact(t *testing.T) {
	root := t.TempDir()
	store := NewFileLoopMemoryStore(root)
	sessionDir := filepath.Join(root, "session-x")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, loopMemoryPRDFile), []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	_, _, err := store.Load(context.Background(), "session-x")
	if err == nil || !strings.Contains(err.Error(), loopMemoryPRDFile) {
		t.Fatalf("expected corrupted artifact error, got %v", err)
	}
}

func TestInMemoryLoopMemoryStoreIsNonDurable(t *testing.T) {
	store := NewInMemoryLoopMemoryStore()
	if store.IsDurable() {
		t.Fatalf("expected non-durable in-memory store")
	}
	if store.Name() != "memory" {
		t.Fatalf("expected memory store name")
	}
}
