package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Startup != "picker" || cfg.SplitRatio != 0.42 || cfg.DBPath == "" {
		t.Fatalf("defaults: %+v", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("config file should be written on first load:", err)
	}
}

func TestLoadRoundtripAndSanitize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.Startup = "last-project"
	cfg.SplitRatio = 0.6
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Startup != "last-project" || got.SplitRatio != 0.6 {
		t.Fatalf("roundtrip: %+v", got)
	}

	os.WriteFile(path, []byte("startup = \"bogus\"\nsplit_ratio = 9.0\n"), 0o644)
	got, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Startup != "picker" || got.SplitRatio != 0.42 {
		t.Fatalf("sanitize: %+v", got)
	}
}
