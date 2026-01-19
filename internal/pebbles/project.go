package pebbles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InitProject initializes the .pebbles directory and cache.
func InitProject(root string) error {
	return InitProjectWithPrefix(root, "")
}

// InitProjectWithPrefix initializes the .pebbles directory and cache with a custom prefix.
func InitProjectWithPrefix(root, prefix string) error {
	if err := os.MkdirAll(PebblesDir(root), 0755); err != nil {
		return fmt.Errorf("create .pebbles dir: %w", err)
	}
	if err := ensureConfig(root, prefix); err != nil {
		return err
	}
	if err := ensureEventsFile(root); err != nil {
		return err
	}
	if err := ensureGitignore(root); err != nil {
		return err
	}
	if err := EnsureCache(root); err != nil {
		return err
	}
	return nil
}

// ensureConfig writes a config file if one does not exist.
func ensureConfig(root, prefix string) error {
	path := ConfigPath(root)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		trimmed = DefaultPrefix(root)
	}
	cfg := Config{Prefix: trimmed}
	return WriteConfig(root, cfg)
}

// ensureEventsFile creates an empty events log if one does not exist.
func ensureEventsFile(root string) error {
	path := EventsPath(root)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create events log: %w", err)
	}
	return file.Close()
}

// ensureGitignore writes a .pebbles/.gitignore if one does not exist.
func ensureGitignore(root string) error {
	path := filepath.Join(PebblesDir(root), ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	content := []byte("pebbles.db\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		return fmt.Errorf("create .pebbles/.gitignore: %w", err)
	}
	return nil
}
