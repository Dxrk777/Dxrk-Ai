// SPDX-License-Identifier: MIT
package dxrk

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

func defaultCtx() tools.Context {
	return tools.Context{Context: context.Background()}
}

func TestRegisterAll(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	expected := []string{
		"detect_agents", "detect_agent", "system_info",
		"list_skills", "run_diagnostic", "read_file",
		"grep_search", "glob_search",
	}
	for _, name := range expected {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
	if reg.Len() != len(expected) {
		t.Errorf("Len() = %d, want %d", reg.Len(), len(expected))
	}
}

func TestDetectAgents_Empty(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("detect_agents")
	if !ok {
		t.Fatal("detect_agents not found")
	}

	home := t.TempDir()
	result, err := tool.Execute(defaultCtx(), map[string]any{"home_dir": home})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if count := r["count"].(int); count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestDetectAgents_WithAgent(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("detect_agents")
	if !ok {
		t.Fatal("detect_agents not found")
	}

	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{"home_dir": home})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	agentsList := r["agents"].([]map[string]any)
	found := false
	for _, a := range agentsList {
		if a["id"] == "claude-code" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("claude-code not found in agents list: %v", agentsList)
	}
}

func TestDetectAgent_Valid(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("detect_agent")
	if !ok {
		t.Fatal("detect_agent not found")
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{
		"agent":    "claude-code",
		"home_dir": t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	if r["agent"] != "claude-code" {
		t.Errorf("agent = %q, want %q", r["agent"], "claude-code")
	}
}

func TestDetectAgent_Unknown(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("detect_agent")
	if !ok {
		t.Fatal("detect_agent not found")
	}

	_, err = tool.Execute(defaultCtx(), map[string]any{
		"agent":    "nonexistent-agent",
		"home_dir": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestDetectAgent_Validate(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("detect_agent")
	if !ok {
		t.Fatal("detect_agent not found")
	}

	if err := tool.Validate(nil); err == nil {
		t.Error("expected validation error for nil input")
	}
	if err := tool.Validate(map[string]any{}); err == nil {
		t.Error("expected validation error for missing agent")
	}
	if err := tool.Validate(map[string]any{"agent": "claude-code"}); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestSystemInfo(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("system_info")
	if !ok {
		t.Fatal("system_info not found")
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	if r["os"] == "" {
		t.Error("os is empty")
	}
	if r["arch"] == "" {
		t.Error("arch is empty")
	}
	if _, ok := r["supported"].(bool); !ok {
		t.Error("supported is not bool")
	}
}

func TestReadFile_Success(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("read_file")
	if !ok {
		t.Fatal("read_file not found")
	}

	tmp := filepath.Join(t.TempDir(), "test.txt")
	content := "hello world"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{"path": tmp})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	if r["content"] != content {
		t.Errorf("content = %q, want %q", r["content"], content)
	}
	if r["size"].(int) != len(content) {
		t.Errorf("size = %d, want %d", r["size"], len(content))
	}
}

func TestReadFile_Directory(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("read_file")
	if !ok {
		t.Fatal("read_file not found")
	}

	_, err = tool.Execute(defaultCtx(), map[string]any{"path": t.TempDir()})
	if err == nil {
		t.Fatal("expected error for directory path")
	}
}

func TestReadFile_Missing(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("read_file")
	if !ok {
		t.Fatal("read_file not found")
	}

	_, err = tool.Execute(defaultCtx(), map[string]any{"path": "/nonexistent/path"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadFile_Validate(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("read_file")
	if !ok {
		t.Fatal("read_file not found")
	}

	if err := tool.Validate(nil); err == nil {
		t.Error("expected validation error for nil input")
	}
	if err := tool.Validate(map[string]any{}); err == nil {
		t.Error("expected validation error for missing path")
	}
	if err := tool.Validate(map[string]any{"path": "/tmp/test"}); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestReadFile_ExceedsMaxSize(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("read_file")
	if !ok {
		t.Fatal("read_file not found")
	}

	tmp := filepath.Join(t.TempDir(), "large.txt")
	data := make([]byte, 100)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = tool.Execute(defaultCtx(), map[string]any{"path": tmp, "max_size": 50})
	if err == nil {
		t.Fatal("expected error for file exceeding max_size")
	}
}

func TestListSkills(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("list_skills")
	if !ok {
		t.Fatal("list_skills not found")
	}

	// Create a temp project with a skill directory
	project := t.TempDir()
	skillDir := filepath.Join(project, ".agents", "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# test"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{"project_dir": project})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	skills := r["skills"].([]map[string]any)
	found := false
	for _, s := range skills {
		if s["name"] == "test-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("test-skill not found in skills list: %v", skills)
	}
}

func TestRunDiagnostic(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("run_diagnostic")
	if !ok {
		t.Fatal("run_diagnostic not found")
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	sys := r["system"].(map[string]any)
	if sys["os"] == "" {
		t.Error("system.os is empty")
	}
	if _, ok := r["agent_count"].(int); !ok {
		t.Error("agent_count missing or not int")
	}
}

func TestRunDiagnostic_NoConfigs(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("run_diagnostic")
	if !ok {
		t.Fatal("run_diagnostic not found")
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{"include_configs": false})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	if _, exists := r["configs"]; exists {
		t.Error("configs should not be present when include_configs=false")
	}
}

func TestGrepSearch_Validate(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("grep_search")
	if !ok {
		t.Fatal("grep_search not found")
	}

	if err := tool.Validate(nil); err == nil {
		t.Error("expected validation error for nil input")
	}
	if err := tool.Validate(map[string]any{}); err == nil {
		t.Error("expected validation error for missing pattern")
	}
}

func TestGlobSearch_Validate(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("glob_search")
	if !ok {
		t.Fatal("glob_search not found")
	}

	if err := tool.Validate(nil); err == nil {
		t.Error("expected validation error for nil input")
	}
	if err := tool.Validate(map[string]any{}); err == nil {
		t.Error("expected validation error for missing pattern")
	}
}

func TestGetHomeDir(t *testing.T) {
	if h := getHomeDir(map[string]any{"home_dir": "/custom/home"}); h != "/custom/home" {
		t.Errorf("getHomeDir() = %q, want %q", h, "/custom/home")
	}
	if h := getHomeDir(nil); h == "" {
		t.Error("getHomeDir(nil) returned empty")
	}
}

func TestGetProjectDir(t *testing.T) {
	if d := getProjectDir(map[string]any{"project_dir": "/custom/project"}); d != "/custom/project" {
		t.Errorf("getProjectDir() = %q, want %q", d, "/custom/project")
	}
	if d := getProjectDir(nil); d == "" {
		t.Error("getProjectDir(nil) returned empty")
	}
}

func TestReadOnly(t *testing.T) {
	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	for _, name := range reg.List() {
		if !name.IsReadOnly() {
			t.Errorf("tool %q is not read-only", name.Name())
		}
	}
}

func TestGrepSearch_RgMissing(t *testing.T) {
	// Test graceful handling when rg is not available
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	_ = os.Setenv("PATH", t.TempDir())

	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("grep_search")
	if !ok {
		t.Fatal("grep_search not found")
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{"pattern": "test"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	if _, hasError := r["error"]; !hasError {
		t.Error("expected error key in result when rg is missing")
	}
	if count := r["count"].(int); count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestGrepSearch_FoundResults(t *testing.T) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("ripgrep not installed, skipping")
	}
	_ = rgPath

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "test.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() error = %v", err)
	}
	if err := RegisterAll(reg, agentReg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	tool, ok := reg.Get("grep_search")
	if !ok {
		t.Fatal("grep_search not found")
	}

	result, err := tool.Execute(defaultCtx(), map[string]any{
		"pattern": "package",
		"path":    tmp,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	r := result.(map[string]any)
	if count := r["count"].(int); count == 0 {
		t.Error("expected at least 1 result")
	}
	var results []string
	switch v := r["results"].(type) {
	case []string:
		results = v
	case []any:
		for _, x := range v {
			results = append(results, x.(string))
		}
	}
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
	if !strings.Contains(results[0], "package") {
		t.Errorf("result does not contain 'package': %v", results[0])
	}
}
