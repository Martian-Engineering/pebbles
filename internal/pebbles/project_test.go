package pebbles

import "testing"

func TestInitProjectWithPrefixWritesConfig(t *testing.T) {
	root := t.TempDir()
	if err := InitProjectWithPrefix(root, "peb"); err != nil {
		t.Fatalf("init project with prefix: %v", err)
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Prefix != "peb" {
		t.Fatalf("expected prefix peb, got %s", cfg.Prefix)
	}
}
