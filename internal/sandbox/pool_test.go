//go:build docker

// SPDX-License-Identifier: MIT
package sandbox

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func hasDocker() bool {
	_, err := exec.LookPath("docker")
	if err != nil {
		return false
	}
	// Verify docker actually works
	err = exec.Command("docker", "info").Run()
	return err == nil
}

func TestNewPool(t *testing.T) {
	if !hasDocker() {
		t.Skip("docker not available")
	}

	pool, err := NewPool(DefaultPoolConfig())
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestExec(t *testing.T) {
	if !hasDocker() {
		t.Skip("docker not available")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	config := ContainerConfig{
		Image:   "alpine:latest",
		Cmd:     []string{"echo", "hello sandbox"},
		Timeout: 30,
	}

	result, err := pool.Exec(context.Background(), config)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello sandbox") {
		t.Errorf("expected stdout to contain 'hello sandbox', got %q", result.Stdout)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestExecScript(t *testing.T) {
	if !hasDocker() {
		t.Skip("docker not available")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	tests := []struct {
		name     string
		lang     Language
		script   string
		want     string
		wantCode int
	}{
		{
			name:     "bash echo",
			lang:     LanguageBash,
			script:   `echo "hello from bash"`,
			want:     "hello from bash",
			wantCode: 0,
		},
		{
			name:     "python hello",
			lang:     LanguagePython,
			script:   `print("hello from python")`,
			want:     "hello from python",
			wantCode: 0,
		},
		{
			name:     "node hello",
			lang:     LanguageNode,
			script:   `console.log("hello from node")`,
			want:     "hello from node",
			wantCode: 0,
		},
		{
			name:     "bash fail",
			lang:     LanguageBash,
			script:   `exit 42`,
			want:     "",
			wantCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pool.ExecScript(context.Background(), tt.script, tt.lang, ExecOptions{})
			if err != nil {
				t.Fatalf("ExecScript failed: %v", err)
			}
			if result.ExitCode != tt.wantCode {
				t.Errorf("exit code: got %d, want %d", result.ExitCode, tt.wantCode)
			}
			if tt.want != "" && !strings.Contains(result.Stdout, tt.want) {
				t.Errorf("stdout: got %q, want %q", result.Stdout, tt.want)
			}
		})
	}
}

func TestExecTimeout(t *testing.T) {
	if !hasDocker() {
		t.Skip("docker not available")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	config := ContainerConfig{
		Image:   "alpine:latest",
		Cmd:     []string{"sh", "-c", "sleep 10; echo done"},
		Timeout: 1, // 1 second timeout
	}

	result, err := pool.Exec(context.Background(), config)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if !result.TimedOut {
		t.Error("expected timed_out = true")
	}
	if result.ExitCode != -1 {
		t.Errorf("expected exit code -1 for timeout, got %d", result.ExitCode)
	}
}

func TestPoolStats(t *testing.T) {
	if !hasDocker() {
		t.Skip("docker not available")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	stats := pool.Stats()
	if stats.TotalExecutions != 0 {
		t.Errorf("expected 0 executions, got %d", stats.TotalExecutions)
	}

	_, _ = pool.Exec(context.Background(), ContainerConfig{
		Image: "alpine:latest",
		Cmd:   []string{"true"},
	})

	stats = pool.Stats()
	if stats.TotalExecutions != 1 {
		t.Errorf("expected 1 execution, got %d", stats.TotalExecutions)
	}
}

func TestImageForLanguage(t *testing.T) {
	tests := []struct {
		lang Language
		want string
	}{
		{LanguageGo, "golang:1.25-alpine"},
		{LanguagePython, "python:3.13-alpine"},
		{LanguageNode, "node:22-alpine"},
		{LanguageBash, "alpine:latest"},
		{LanguageRust, "rust:1.82-alpine"},
		{LanguageTypeScript, "node:22-alpine"},
		{Language("unknown"), "alpine:latest"},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			got := ImageForLanguage(tt.lang)
			if got != tt.want {
				t.Errorf("ImageForLanguage(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestPoolNewPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test")
	}

	pool, err := NewPool(DefaultPoolConfig())
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
	if pool.config.MaxContainers != 10 {
		t.Errorf("MaxContainers = %d, want 10", pool.config.MaxContainers)
	}
}

func TestPoolExec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	result, err := pool.Exec(context.Background(), ContainerConfig{
		Image: "alpine:latest",
		Cmd:   []string{"echo", "hello sandbox"},
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello sandbox") {
		t.Errorf("stdout = %q, want 'hello sandbox'", result.Stdout)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestPoolExecScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	tests := []struct {
		name     string
		lang     Language
		script   string
		want     string
		wantCode int
	}{
		{name: "bash echo", lang: LanguageBash, script: `echo "hello from bash"`, want: "hello from bash", wantCode: 0},
		{name: "python hello", lang: LanguagePython, script: `print("hello from python")`, want: "hello from python", wantCode: 0},
		{name: "node hello", lang: LanguageNode, script: `console.log("hello from node")`, want: "hello from node", wantCode: 0},
		{name: "bash fail", lang: LanguageBash, script: `exit 42`, want: "", wantCode: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pool.ExecScript(context.Background(), tt.script, tt.lang, ExecOptions{})
			if err != nil {
				t.Fatalf("ExecScript failed: %v", err)
			}
			if result.ExitCode != tt.wantCode {
				t.Errorf("exit code = %d, want %d", result.ExitCode, tt.wantCode)
			}
			if tt.want != "" && !strings.Contains(result.Stdout, tt.want) {
				t.Errorf("stdout = %q, want %q", result.Stdout, tt.want)
			}
		})
	}
}

func TestPoolExecTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	result, err := pool.Exec(context.Background(), ContainerConfig{
		Image:   "alpine:latest",
		Cmd:     []string{"sh", "-c", "sleep 10; echo done"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if !result.TimedOut {
		t.Error("expected TimedOut=true")
	}
	if result.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1", result.ExitCode)
	}
}

func TestPoolStatsIncrement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	stats := pool.Stats()
	if stats.TotalExecutions != 0 {
		t.Fatalf("expected 0 executions initially, got %d", stats.TotalExecutions)
	}

	for i := 0; i < 3; i++ {
		_, _ = pool.Exec(context.Background(), ContainerConfig{
			Image: "alpine:latest",
			Cmd:   []string{"true"},
		})
	}

	stats = pool.Stats()
	if stats.TotalExecutions != 3 {
		t.Errorf("TotalExecutions = %d, want 3", stats.TotalExecutions)
	}

	_, _ = pool.Exec(context.Background(), ContainerConfig{
		Image: "alpine:latest",
		Cmd:   []string{"sh", "-c", "exit 1"},
	})

	stats = pool.Stats()
	if stats.TotalExecutions != 4 {
		t.Errorf("TotalExecutions = %d, want 4", stats.TotalExecutions)
	}
	if stats.FailedExecutions != 1 {
		t.Errorf("FailedExecutions = %d, want 1", stats.FailedExecutions)
	}
}

func TestPoolSessionLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer func() { _ = pool.Close() }()

	ctx := context.Background()
	sessionID := "test-session-lifecycle"

	containerID, err := pool.GetContainer(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}
	if containerID == "" {
		t.Fatal("expected non-empty container ID")
	}

	cid2, err := pool.GetContainer(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetContainer (second call) failed: %v", err)
	}
	if cid2 != containerID {
		t.Errorf("second GetContainer returned different ID: %q vs %q", cid2, containerID)
	}

	result, err := pool.ExecInSession(ctx, sessionID, "", []string{"echo", "hello session"})
	if err != nil {
		t.Fatalf("ExecInSession failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello session") {
		t.Errorf("stdout = %q, want 'hello session'", result.Stdout)
	}

	result, err = pool.ExecInSession(ctx, sessionID, "/tmp", []string{"pwd"})
	if err != nil {
		t.Fatalf("ExecInSession with workDir failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "/tmp") {
		t.Errorf("stdout = %q, want '/tmp'", result.Stdout)
	}

	_, err = pool.ExecInSession(ctx, "nonexistent", "", []string{"echo", "hi"})
	if err == nil {
		t.Error("expected error for nonexistent session")
	}

	err = pool.ReleaseContainer(ctx, sessionID)
	if err != nil {
		t.Fatalf("ReleaseContainer failed: %v", err)
	}

	err = pool.ReleaseContainer(ctx, sessionID)
	if err == nil {
		t.Error("expected error for releasing already-released session")
	}
}

func TestPoolClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker test")
	}

	pool, err := NewPool(PoolConfig{
		MaxContainers: 2,
		DefaultImage:  "alpine:latest",
		DockerCmd:     "docker",
	})
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	ctx := context.Background()
	_, err = pool.GetContainer(ctx, "test-close-session")
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}

	err = pool.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	err = pool.Close()
	if err != nil {
		t.Errorf("second Close should not error: %v", err)
	}
}

func TestCommandForLanguage(t *testing.T) {
	tests := []struct {
		lang Language
		file string
		want []string
	}{
		{LanguageGo, "main.go", []string{"go", "run", "main.go"}},
		{LanguagePython, "main.py", []string{"python3", "main.py"}},
		{LanguageNode, "main.js", []string{"node", "main.js"}},
		{LanguageBash, "script.sh", []string{"sh", "script.sh"}},
		{LanguageRust, "main.rs", []string{"sh", "-c", "rustc main.rs -o /tmp/out && /tmp/out"}},
		{LanguageTypeScript, "main.ts", []string{"npx", "ts-node", "main.ts"}},
		{Language(""), "script.sh", []string{"sh", "script.sh"}},
		{Language("unknown"), "script.sh", []string{"sh", "script.sh"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			got := CommandForLanguage(tt.lang, tt.file)
			if len(got) != len(tt.want) {
				t.Fatalf("CommandForLanguage(%q, %q) = %v, want %v", tt.lang, tt.file, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("CommandForLanguage(%q, %q) = %v, want %v", tt.lang, tt.file, got, tt.want)
				}
			}
		})
	}
}
