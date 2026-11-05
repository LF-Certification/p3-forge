// Package config reads and writes $XDG_CONFIG_HOME/lfed/config.yaml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/credstore"
)

const fileName = "config.yaml"

// Config is the on-disk document.
type Config struct {
	CurrentContext string `yaml:"current_context,omitempty"`
}

// Dir returns the config directory. LFED_CONFIG_DIR overrides it for tests.
func Dir() (string, error) {
	if d := os.Getenv("LFED_CONFIG_DIR"); d != "" {
		return d, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lfed"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(home, ".config", "lfed"), nil
}

// Load reads the config file. A missing file yields an empty Config.
func Load() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Join(dir, fileName), err)
	}
	return &cfg, nil
}

// Save replaces the config file atomically with mode 0600. It creates the
// directory with mode 0700 and narrows a wider existing directory on unix.
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := credstore.MkdirPrivate(dir); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := credstore.WriteOwnerOnlyFile(filepath.Join(dir, fileName), data); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
