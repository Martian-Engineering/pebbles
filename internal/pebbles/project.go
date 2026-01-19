package pebbles

import (
	"fmt"
	"os"
)

// InitProject initializes the .pebbles directory and cache.
func InitProject(root string) error {
	if err := os.MkdirAll(PebblesDir(root), 0755); err != nil {
		return fmt.Errorf("create .pebbles dir: %w", err)
	}
	if err := ensureConfig(root); err != nil {
		return err
	}
	if err := ensureEventsFile(root); err != nil {
		return err
	}
	if err := EnsureCache(root); err != nil {
		return err
	}
	return nil
}

// ensureConfig writes a config file if one does not exist.
func ensureConfig(root string) error {
	path := ConfigPath(root)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	cfg := Config{Prefix: DefaultPrefix(root)}
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
