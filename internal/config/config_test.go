// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Providers) == 0 {
		t.Fatal("expected at least one provider")
	}
	if cfg.Sandbox == nil {
		t.Fatal("expected sandbox config")
	}
	if cfg.Autonomy == nil {
		t.Fatal("expected autonomy config")
	}
}

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dxrk.yaml")

	cfg := Default()
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Project.Name != "my-project" {
		t.Fatalf("expected 'my-project', got %q", loaded.Project.Name)
	}
	if len(loaded.Providers) != 4 {
		t.Fatalf("expected 4 providers, got %d", len(loaded.Providers))
	}
}

func TestLoadNonExistentCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-such-file.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load non-existent: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected default config file to be created")
	}
}

func TestProviderByName(t *testing.T) {
	cfg := Default()
	if len(cfg.Providers) == 0 {
		t.Fatal("no providers")
	}
	if cfg.Providers[0].Name != "claude" {
		t.Fatalf("expected 'claude', got %q", cfg.Providers[0].Name)
	}
}

func TestSandboxDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Sandbox.DefaultImage != "ubuntu:22.04" {
		t.Fatalf("expected 'ubuntu:22.04', got %q", cfg.Sandbox.DefaultImage)
	}
	if cfg.Sandbox.TimeoutSec != 120 {
		t.Fatalf("expected 120, got %d", cfg.Sandbox.TimeoutSec)
	}
}

func TestAutonomyDefaults(t *testing.T) {
	cfg := Default()
	if !cfg.Autonomy.SelfUpdate {
		t.Fatal("expected self_update enabled by default")
	}
	if !cfg.Autonomy.SelfVerify {
		t.Fatal("expected self_verify enabled by default")
	}
	if !cfg.Autonomy.SelfLearn {
		t.Fatal("expected self_learn enabled by default")
	}
	if cfg.Autonomy.IntervalSec != 300 {
		t.Fatalf("expected interval 300, got %d", cfg.Autonomy.IntervalSec)
	}
}
