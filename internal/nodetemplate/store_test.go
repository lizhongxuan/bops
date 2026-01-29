package nodetemplate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorePutGetList(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)
	_, err := store.Put("demo", Template{
		Name:        "demo",
		Category:    "actions",
		Description: "test",
		Tags:        []string{"ops"},
		Node: NodeSpec{
			Type:   "action",
			Name:   "install",
			Action: "pkg.install",
			With: map[string]any{
				"name": "nginx",
			},
		},
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	item, _, err := store.Get("demo")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if item.Name != "demo" {
		t.Fatalf("expected name demo, got %q", item.Name)
	}
	list, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}
	if list[0].Node.Action != "pkg.install" {
		t.Fatalf("expected node action pkg.install, got %q", list[0].Node.Action)
	}
	if err := store.Delete("demo"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected file removed")
	}
}
