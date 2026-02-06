package scriptstore

import "testing"

func TestScriptStorePutGetList(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	_, err := store.Put("demo", Script{
		Name:        "demo",
		Language:    "shell",
		Description: "sample",
		Tags:        []string{"ops"},
		Content:     "echo hi",
	})
	if err != nil {
		t.Fatalf("put script: %v", err)
	}

	script, _, err := store.Get("demo")
	if err != nil {
		t.Fatalf("get script: %v", err)
	}
	if script.Name != "demo" || script.Language != "shell" {
		t.Fatalf("unexpected script: %+v", script)
	}

	items, err := store.List()
	if err != nil {
		t.Fatalf("list scripts: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 script, got %d", len(items))
	}
	if items[0].Name != "demo" {
		t.Fatalf("unexpected list entry: %+v", items[0])
	}
}

func TestScriptStoreRejectsInvalidLanguage(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	_, err := store.Put("bad", Script{
		Name:     "bad",
		Language: "ruby",
		Content:  "puts 'hi'",
	})
	if err == nil {
		t.Fatalf("expected error for invalid language")
	}
}
