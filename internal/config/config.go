package config

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Startup    string  `toml:"startup"`     // "picker" | "last-project"
	SplitRatio float64 `toml:"split_ratio"` // left panel fraction, 0.2–0.8
	DBPath     string  `toml:"db_path"`
}

func Default() Config {
	return Config{
		Startup:    "picker",
		SplitRatio: 0.42,
		DBPath:     filepath.Join(DataDir(), "tskr.db"),
	}
}

// DataDir is where the database and backups live.
func DataDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "tskr")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "tskr")
}

// Path is the default config file location.
func Path() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "tskr", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tskr", "config.toml")
}

// Load reads the config, writing defaults on first run. Invalid values
// fall back to defaults rather than erroring.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, Save(path, cfg)
	}
	if err != nil {
		return cfg, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Startup != "picker" && cfg.Startup != "last-project" {
		cfg.Startup = "picker"
	}
	if cfg.SplitRatio < 0.2 || cfg.SplitRatio > 0.8 {
		cfg.SplitRatio = 0.42
	}
	if cfg.DBPath == "" {
		cfg.DBPath = Default().DBPath
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
