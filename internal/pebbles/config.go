package pebbles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPrefix derives a prefix from the project directory name.
func DefaultPrefix(root string) string {
	base := filepath.Base(root)
	if base == "." || base == string(filepath.Separator) {
		return "pb"
	}
	return base
}

// LoadConfig reads the Pebbles config from disk.
func LoadConfig(root string) (Config, error) {
	path := ConfigPath(root)
	// Read and parse the JSON config file.
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Prefix == "" {
		return Config{}, fmt.Errorf("config missing prefix")
	}
	return cfg, nil
}

// WriteConfig writes the Pebbles config to disk.
func WriteConfig(root string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := ConfigPath(root)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
