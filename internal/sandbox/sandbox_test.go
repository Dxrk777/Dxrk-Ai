//go:build docker

// SPDX-License-Identifier: MIT
package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()
	if cfg.MaxContainers != 10 {
		t.Errorf("MaxContainers = %d, want 10", cfg.MaxContainers)
	}
	if cfg.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout = %v, want 5m", cfg.IdleTimeout)
	}
	if cfg.DefaultImage != golangAlpineImage {
		t.Errorf("DefaultImage = %q, want %q", cfg.DefaultImage, golangAlpineImage)
	}
	if cfg.DockerCmd != "docker" {
		t.Errorf("DockerCmd = %q, want docker", cfg.DockerCmd)
	}
}

func TestScriptFileName(t *testing.T) {
	tests := []struct {
		lang Language
		want string
	}{
		{LanguageGo, "main.go"},
		{LanguagePython, "main.py"},
		{LanguageNode, "main.js"},
		{LanguageBash, "script.sh"},
		{LanguageRust, "main.rs"},
		{LanguageTypeScript, "main.ts"},
		{Language(""), "script.sh"},
		{Language("unknown"), "script.sh"},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			got := scriptFileName(tt.lang)
			if got != tt.want {
				t.Errorf("scriptFileName(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestHasFile(t *testing.T) {
	dir := t.TempDir()

	if hasFile(dir, "nonexistent.txt") {
		t.Error("hasFile should return false for nonexistent file")
	}

	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if !hasFile(dir, "test.txt") {
		t.Error("hasFile should return true for existing file")
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	if hasFile(dir, "subdir") {
		t.Error("hasFile should return false for directories")
	}
}

func TestDetectTestCommand(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string)
		want  string
	}{
		{
			name: "go mod",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			want: "go test ./... 2>&1",
		},
		{
			name: "cargo toml",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)
			},
			want: "cargo test 2>&1",
		},
		{
			name: "package json",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
			},
			want: "npm test 2>&1",
		},
		{
			name: "pyproject toml",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)
			},
			want: "python3 -m pytest 2>&1 || python3 -m unittest 2>&1",
		},
		{
			name: "requirements txt",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("pytest"), 0644)
			},
			want: "python3 -m pytest 2>&1 || python3 -m unittest 2>&1",
		},
		{
			name:  "no project files",
			setup: func(dir string) {},
			want:  "echo 'No test command detected' && exit 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			got := detectTestCommand(dir)
			if got != tt.want {
				t.Errorf("detectTestCommand(%q) = %q, want %q", dir, got, tt.want)
			}
		})
	}
}

func TestDetectBuildCommand(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string)
		want  string
	}{
		{
			name: "go mod",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			want: "go build ./... 2>&1",
		},
		{
			name: "cargo toml",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)
			},
			want: "cargo build 2>&1",
		},
		{
			name: "package json",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
			},
			want: "npm run build 2>&1",
		},
		{
			name: "makefile",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:"), 0644)
			},
			want: "make 2>&1",
		},
		{
			name:  "no project files",
			setup: func(dir string) {},
			want:  "echo 'No build command detected' && exit 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			got := detectBuildCommand(dir)
			if got != tt.want {
				t.Errorf("detectBuildCommand(%q) = %q, want %q", dir, got, tt.want)
			}
		})
	}
}

func TestTestImageForProject(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string)
		want  string
	}{
		{
			name: "go mod",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			want: golangAlpineImage,
		},
		{
			name: "cargo toml",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)
			},
			want: "rust:1.82-alpine",
		},
		{
			name: "package json",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
			},
			want: nodeAlpineImage,
		},
		{
			name: "pyproject toml",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)
			},
			want: "python:3.13-alpine",
		},
		{
			name: "requirements txt",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("pytest"), 0644)
			},
			want: "python:3.13-alpine",
		},
		{
			name:  "no project files",
			setup: func(dir string) {},
			want:  defaultAlpineImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			got := testImageForProject(dir)
			if got != tt.want {
				t.Errorf("testImageForProject(%q) = %q, want %q", dir, got, tt.want)
			}
		})
	}
}

func TestRegisterTools(t *testing.T) {
	reg := tools.New()
	err := RegisterTools(reg)
	if err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	expectedNames := []string{"sandbox_run_code", "sandbox_run_tests", "sandbox_build"}
	if reg.Len() != len(expectedNames) {
		t.Errorf("expected %d tools, got %d", len(expectedNames), reg.Len())
	}

	for _, name := range expectedNames {
		tool, ok := reg.Get(name)
		if !ok {
			t.Fatalf("expected tool %q to be registered", name)
		}
		if tool.Description() == "" {
			t.Errorf("tool %q has empty description", name)
		}
		if tool.InputSchema() == nil {
			t.Errorf("tool %q has nil input schema", name)
		}
		schema := tool.InputSchema()
		ttype, ok := schema["type"]
		if !ok || ttype != "object" {
			t.Errorf("tool %q input schema type = %v, want object", name, ttype)
		}
		props, ok := schema["properties"]
		if !ok || props == nil {
			t.Errorf("tool %q input schema has no properties", name)
		}
	}
}

func TestRegisterToolsDuplicate(t *testing.T) {
	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatalf("first RegisterTools failed: %v", err)
	}
	if err := RegisterTools(reg); err == nil {
		t.Error("expected error on duplicate registration")
	}
}

func TestPoolStatsFreshPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test in short mode")
	}

	pool, err := NewPool(DefaultPoolConfig())
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	stats := pool.Stats()
	if stats.TotalContainers != 0 {
		t.Errorf("TotalContainers = %d, want 0", stats.TotalContainers)
	}
	if stats.ActiveContainers != 0 {
		t.Errorf("ActiveContainers = %d, want 0", stats.ActiveContainers)
	}
	if stats.IdleContainers != 0 {
		t.Errorf("IdleContainers = %d, want 0", stats.IdleContainers)
	}
	if stats.TotalExecutions != 0 {
		t.Errorf("TotalExecutions = %d, want 0", stats.TotalExecutions)
	}
	if stats.FailedExecutions != 0 {
		t.Errorf("FailedExecutions = %d, want 0", stats.FailedExecutions)
	}
}

func TestHasFilePrioritizesOrder(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	got := detectTestCommand(dir)
	if !strings.Contains(got, "go test") {
		t.Errorf("expected go test detection, got %q", got)
	}
}

func TestHasFileDirectoryNotFile(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "go.mod"), 0755)

	if hasFile(dir, "go.mod") {
		t.Error("hasFile should return false for a directory named go.mod")
	}
}
