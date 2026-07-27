// SPDX-License-Identifier: MIT
package checker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultCheckerConfig(t *testing.T) {
	home := t.TempDir()
	got := DefaultCheckerConfig(home)
	want := filepath.Join(home, ".config", "dxrk", "checker.json")
	if got != want {
		t.Errorf("DefaultCheckerConfig(%q) = %q, want %q", home, got, want)
	}
}

func TestInject_CreatesConfigFile(t *testing.T) {
	home := t.TempDir()
	result, err := Inject(home)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Error("Inject() result.Changed = false, want true (new file)")
	}
	if len(result.Files) != 1 {
		t.Fatalf("Inject() result.Files = %v, want [checker.json]", result.Files)
	}

	expectedPath := DefaultCheckerConfig(home)
	if result.Files[0] != expectedPath {
		t.Errorf("Inject() result.Files[0] = %q, want %q", result.Files[0], expectedPath)
	}

	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}

	var cfg checkerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("json.Unmarshal(config) error = %v", err)
	}

	if !cfg.DenyByDefault {
		t.Error("config.deny_by_default = false, want true")
	}
	if len(cfg.Rules) == 0 {
		t.Fatal("config.rules is empty, expected deny rules")
	}

	hasEnvRule := false
	for _, r := range cfg.Rules {
		if r.Target == "*.env" && r.Action == "read" && !r.Allow {
			hasEnvRule = true
		}
	}
	if !hasEnvRule {
		t.Error("expected a deny rule for read *.env")
	}
}

func TestInject_Idempotent(t *testing.T) {
	home := t.TempDir()
	result1, err := Inject(home)
	if err != nil {
		t.Fatalf("first Inject() error = %v", err)
	}
	if !result1.Changed {
		t.Error("first Inject() should report Changed=true")
	}

	result2, err := Inject(home)
	if err != nil {
		t.Fatalf("second Inject() error = %v", err)
	}
	if result2.Changed {
		t.Error("second Inject() should report Changed=false (no-op)")
	}
}

func TestInject_NewHomeDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), "deep", "nonexistent")
	result, err := Inject(home)
	if err != nil {
		t.Fatalf("Inject() into new deep path error = %v", err)
	}
	if !result.Changed {
		t.Error("Inject() result.Changed = false, want true")
	}
	if _, err := os.Stat(DefaultCheckerConfig(home)); os.IsNotExist(err) {
		t.Error("DefaultCheckerConfig() does not exist after Inject()")
	}
}

func TestLoadRules_ReturnsDenyByDefault(t *testing.T) {
	home := t.TempDir()
	if _, err := Inject(home); err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	rules, denyByDefault, err := LoadRules(home)
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if !denyByDefault {
		t.Error("LoadRules() denyByDefault = false, want true")
	}
	if len(rules) == 0 {
		t.Fatal("LoadRules() returned no rules")
	}
}

func TestLoadRules_NoFileReturnsNil(t *testing.T) {
	home := t.TempDir()
	rules, denyByDefault, err := LoadRules(home)
	if err != nil {
		t.Fatalf("LoadRules() on missing file error = %v", err)
	}
	if rules != nil {
		t.Errorf("LoadRules() rules = %v, want nil", rules)
	}
	if !denyByDefault {
		t.Error("LoadRules() denyByDefault on missing file = false, want true")
	}
}
