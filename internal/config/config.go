package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for mneme.
type Config struct {
	NotesDir string `yaml:"notes_dir"`
	Database string `yaml:"database"`
	Review   struct {
		Tags    []string `yaml:"tags"`
		Match   string   `yaml:"match"`
		Exclude []string `yaml:"exclude"`
	} `yaml:"review"`
}

// DefaultConfig returns the built-in defaults.
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.NotesDir = "~/Documents/RedHatNotes/Resources"
	cfg.Review.Match = "any"
	return cfg
}

// XDGConfigPath returns the XDG config path for the mneme config file.
func XDGConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(base, "mneme", "config.yaml")
}

// XDGStatePath returns the XDG state path for the mneme database.
func XDGStatePath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(homeDir(), ".local", "state")
	}
	return filepath.Join(base, "mneme", "mneme.db")
}

// Load reads and parses the config file at path. If the file does not exist,
// it returns DefaultConfig() with no error. Unknown YAML keys produce a warning
// to stderr but are not fatal.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	// Decode with strict mode first to detect unknown keys; if it fails, do a
	// lenient decode and warn.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	knownKeys := map[string]bool{
		"notes_dir": true,
		"database":  true,
		"review":    true,
	}
	for k := range raw {
		if !knownKeys[k] {
			fmt.Fprintf(os.Stderr, "warning: unknown config key %q in %s\n", k, path)
		}
	}

	// Now do the real unmarshal into the Config struct (lenient).
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Apply defaults for fields that were not set.
	if cfg.Review.Match == "" {
		cfg.Review.Match = "any"
	}

	return cfg, nil
}

// ExpandPath expands a leading ~ to $HOME and then expands environment variables.
func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		p = filepath.Join(homeDir(), p[2:])
	} else if p == "~" {
		p = homeDir()
	}
	return os.ExpandEnv(p)
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return h
}
