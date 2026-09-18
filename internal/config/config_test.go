package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := Load(path)
	if err != nil { t.Fatal(err) }
	if !cfg.Sync.PreferSymlinks || !cfg.Sync.ConfirmChanges || cfg.DefaultProfile != "default" || cfg.TUI.Theme != "auto" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	data := []byte(`default_profile = "backend"

[sync]
prefer_symlinks = false
confirm_changes = false

[updates]
check_on_start = true

[tui]
show_hidden = true
theme = "mono"
`)
	if err := os.WriteFile(path, data, 0o600); err != nil { t.Fatal(err) }
	cfg, err = Load(path)
	if err != nil { t.Fatal(err) }
	if cfg.DefaultProfile != "backend" || cfg.Sync.PreferSymlinks || cfg.Sync.ConfirmChanges || !cfg.Updates.CheckOnStart || !cfg.TUI.ShowHidden || cfg.TUI.Theme != "mono" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}
