package pebbles

import "path/filepath"

// PebblesDir returns the .pebbles directory path for a project root.
func PebblesDir(root string) string {
	return filepath.Join(root, ".pebbles")
}

// EventsPath returns the events.jsonl path for a project root.
func EventsPath(root string) string {
	return filepath.Join(PebblesDir(root), "events.jsonl")
}

// ConfigPath returns the config.json path for a project root.
func ConfigPath(root string) string {
	return filepath.Join(PebblesDir(root), "config.json")
}

// DBPath returns the SQLite cache path for a project root.
func DBPath(root string) string {
	return filepath.Join(PebblesDir(root), "pebbles.db")
}
