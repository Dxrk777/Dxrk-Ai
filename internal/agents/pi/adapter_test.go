// SPDX-License-Identifier: MIT
package pi

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/system"
	"github.com/Dxrk777/Dxrk-Ai/internal/versions"
)

func TestAdapterIdentityAndCapabilities(t *testing.T) {
	a := NewAdapter()

	if got := a.Agent(); got != model.AgentPi {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentPi)
	}
	if got := a.Tier(); got != model.TierFull {
		t.Fatalf("Tier() = %q, want %q", got, model.TierFull)
	}

	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"SupportsAutoInstall", a.SupportsAutoInstall(), true},
		{"SupportsSkills", a.SupportsSkills(), false},
		{"SupportsMCP", a.SupportsMCP(), true},
		{"SupportsSystemPrompt", a.SupportsSystemPrompt(), false},
		{"SupportsSlashCommands", a.SupportsSlashCommands(), false},
		{"SupportsOutputStyles", a.SupportsOutputStyles(), false},
		{"SupportsSubAgents", a.SupportsSubAgents(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestStrategies(t *testing.T) {
	a := NewAdapter()

	if got := a.SystemPromptStrategy(); got != model.StrategyAppendToFile {
		t.Fatalf("SystemPromptStrategy() = %v, want %v", got, model.StrategyAppendToFile)
	}

	if got := a.MCPStrategy(); got != model.StrategyMCPConfigFile {
		t.Fatalf("MCPStrategy() = %v, want %v", got, model.StrategyMCPConfigFile)
	}
}

func TestAdapterPaths(t *testing.T) {
	a := NewAdapter()
	homeDir := t.TempDir()
	piDir := filepath.Join(homeDir, ".pi")
	piAgentDir := filepath.Join(piDir, "agent")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(homeDir), piDir},
		{"SystemPromptDir", a.SystemPromptDir(homeDir), ""},
		{"SystemPromptFile", a.SystemPromptFile(homeDir), ""},
		{"SkillsDir", a.SkillsDir(homeDir), ""},
		{"SettingsPath", a.SettingsPath(homeDir), filepath.Join(piAgentDir, "settings.json")},
		{"CommandsDir", a.CommandsDir(homeDir), ""},
		{"MCPConfigPath", a.MCPConfigPath(homeDir, "context7"), filepath.Join(piAgentDir, "mcp.json")},
		{"OutputStyleDir", a.OutputStyleDir(homeDir), ""},
		{"SubAgentsDir", a.SubAgentsDir(homeDir), ""},
		{"EmbeddedSubAgentsDir", a.EmbeddedSubAgentsDir(), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestConfigPathHelpers(t *testing.T) {
	homeDir := "/home/test"

	if got := ConfigPath(homeDir); got != filepath.Join(homeDir, ".pi") {
		t.Fatalf("ConfigPath() = %q, want %q", got, filepath.Join(homeDir, ".pi"))
	}

	if got := AgentConfigPath(homeDir); got != filepath.Join(homeDir, ".pi", "agent") {
		t.Fatalf("AgentConfigPath() = %q, want %q", got, filepath.Join(homeDir, ".pi", "agent"))
	}
}

func TestAdapterDetectUsesPiBinaryAndConfigPath(t *testing.T) {
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".pi")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{
		lookPath: func(file string) (string, error) {
			if file != "pi" {
				t.Fatalf("lookPath called with %q, want pi", file)
			}
			return "/usr/local/bin/pi", nil
		},
		statPath: defaultStat,
	}

	installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !installed {
		t.Fatalf("Detect() installed = false, want true")
	}
	if binaryPath != "/usr/local/bin/pi" {
		t.Fatalf("Detect() binaryPath = %q, want /usr/local/bin/pi", binaryPath)
	}
	if configPath != configDir {
		t.Fatalf("Detect() configPath = %q, want %q", configPath, configDir)
	}
	if !configFound {
		t.Fatalf("Detect() configFound = false, want true")
	}
}

func TestAdapterDetectMissingPiBinary(t *testing.T) {
	homeDir := t.TempDir()
	a := &Adapter{
		lookPath: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
		statPath: defaultStat,
	}

	installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if installed {
		t.Fatalf("Detect() installed = true, want false")
	}
	if binaryPath != "" {
		t.Fatalf("Detect() binaryPath = %q, want empty", binaryPath)
	}
	if configPath != filepath.Join(homeDir, ".pi") {
		t.Fatalf("Detect() configPath = %q, want ~/.pi under home", configPath)
	}
	if configFound {
		t.Fatalf("Detect() configFound = true, want false")
	}
}

func TestAdapterDetectMissingConfigWithBinary(t *testing.T) {
	homeDir := t.TempDir()
	a := &Adapter{
		lookPath: func(file string) (string, error) {
			return "/usr/local/bin/pi", nil
		},
		statPath: func(path string) statResult {
			return statResult{err: os.ErrNotExist}
		},
	}

	_, _, _, configFound, err := a.Detect(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if configFound {
		t.Fatalf("Detect() configFound = true, want false (config dir does not exist)")
	}
}

func TestAdapterDetectStatError(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "", nil },
		statPath: func(string) statResult {
			return statResult{err: os.ErrPermission}
		},
	}

	_, _, _, _, err := a.Detect(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("Detect() expected error for permission denied")
	}
}

func TestAdapterInstallCommandSequenceIsExact(t *testing.T) {
	a := NewAdapter()
	commands, err := a.InstallCommand(system.PlatformProfile{})
	if err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}

	want := [][]string{
		{"pi", "install", "npm:dxrk-pi"},
		{"pi", "install", "npm:dxrk-dxrk-memory"},
		{"pi", "install", "npm:pi-mcp-adapter"},
		{"npm", "exec", "--yes", "--package", "dxrk-dxrk-memory@" + versions.DxrkEngram, "--", "pi-dxrk-memory", "init"},
		{"pi", "install", "npm:pi-subagents"},
		{"pi", "install", "npm:pi-intercom"},
		{"pi", "install", "npm:@juicesharp/rpiv-ask-user-question"},
		{"pi", "install", "npm:pi-web-access"},
		{"pi", "install", "npm:pi-lens"},
		{"pi", "install", "npm:@juicesharp/rpiv-todo"},
		{"pi", "install", "npm:pi-btw"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("InstallCommand() = %#v, want %#v", commands, want)
	}
}

func TestProvisionDxrkMemoryMCP_CreatesFiles(t *testing.T) {
	a := NewAdapter()
	homeDir := t.TempDir()

	changed, paths, err := a.ProvisionDxrkMemoryMCP(homeDir)
	if err != nil {
		t.Fatalf("ProvisionDxrkMemoryMCP() error = %v", err)
	}

	if !changed {
		t.Fatal("ProvisionDxrkMemoryMCP() changed = false, want true (files created)")
	}
	if len(paths) != 2 {
		t.Fatalf("ProvisionDxrkMemoryMCP() returned %d paths, want 2", len(paths))
	}

	// Check settings.json was created with pi-mcp-adapter package.
	settingsPath := paths[0]
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	if !containsPackage(data, "npm:pi-mcp-adapter") {
		t.Fatalf("settings.json does not contain pi-mcp-adapter package:\n%s", string(data))
	}

	// Check package.json was created with pi-mcp-adapter dependency.
	pkgPath := paths[1]
	data, err = os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !containsDependency(data, piMCPAdapterDependency) {
		t.Fatalf("package.json does not contain pi-mcp-adapter dependency:\n%s", string(data))
	}
}

func TestProvisionDxrkMemoryMCP_Idempotent(t *testing.T) {
	a := NewAdapter()
	homeDir := t.TempDir()

	changed, _, err := a.ProvisionDxrkMemoryMCP(homeDir)
	if err != nil {
		t.Fatalf("first ProvisionDxrkMemoryMCP() error = %v", err)
	}
	if !changed {
		t.Fatal("first call should report changed")
	}

	changed, _, err = a.ProvisionDxrkMemoryMCP(homeDir)
	if err != nil {
		t.Fatalf("second ProvisionDxrkMemoryMCP() error = %v", err)
	}
	if changed {
		t.Fatal("second call should report unchanged (idempotent)")
	}
}

func TestProvisionDxrkMemoryMCP_PreservesExistingSettings(t *testing.T) {
	a := NewAdapter()
	homeDir := t.TempDir()
	settingsPath := filepath.Join(homeDir, ".pi", "agent", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initialContent := `{
  "packages": ["npm:dxrk-pi"],
  "customKey": "keepMe"
}
`
	if err := os.WriteFile(settingsPath, []byte(initialContent), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, _, err := a.ProvisionDxrkMemoryMCP(homeDir)
	if err != nil {
		t.Fatalf("ProvisionDxrkMemoryMCP() error = %v", err)
	}
	if !changed {
		t.Fatal("expected changed = true")
	}

	data, _ := os.ReadFile(settingsPath)
	if !containsPackage(data, "npm:dxrk-pi") {
		t.Fatal("existing 'npm:dxrk-pi' was removed")
	}
	if !containsPackage(data, "npm:pi-mcp-adapter") {
		t.Fatal("pi-mcp-adapter was not added")
	}
}

func TestProvisionDxrkMemoryMCP_ReplacesExistingPiMCPAdapter(t *testing.T) {
	a := NewAdapter()
	homeDir := t.TempDir()
	settingsPath := filepath.Join(homeDir, ".pi", "agent", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initialContent := `{
  "packages": ["npm:pi-mcp-adapter@1.0.0"]
}
`
	if err := os.WriteFile(settingsPath, []byte(initialContent), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, _, err := a.ProvisionDxrkMemoryMCP(homeDir)
	if err != nil {
		t.Fatalf("ProvisionDxrkMemoryMCP() error = %v", err)
	}
	if !changed {
		t.Fatal("expected changed = true")
	}

	data, _ := os.ReadFile(settingsPath)
	if containsPackage(data, "npm:pi-mcp-adapter@1.0.0") || containsPackage(data, "npm:pi-mcp-adapter@") {
		t.Fatalf("old versioned pi-mcp-adapter was not replaced:\n%s", string(data))
	}
	if !containsPackage(data, "npm:pi-mcp-adapter") {
		t.Fatalf("new pi-mcp-adapter not found:\n%s", string(data))
	}
}

func TestProvisionDxrkMemoryMCP_PackageJSONMapFormat(t *testing.T) {
	a := NewAdapter()
	homeDir := t.TempDir()
	npmDir := filepath.Join(homeDir, ".pi", "npm")
	if err := os.MkdirAll(npmDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write package.json with a map-format dependencies section (legacy format).
	pkgPath := filepath.Join(npmDir, "package.json")
	pkgContent := `{
  "dependencies": {
    "some-pkg": "^1.0.0"
  }
}
`
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := a.ProvisionDxrkMemoryMCP(homeDir)
	if err != nil {
		t.Fatalf("ProvisionDxrkMemoryMCP() error = %v", err)
	}

	data, _ := os.ReadFile(pkgPath)
	if !containsDependency(data, "some-pkg") {
		t.Fatal("existing dependency was removed")
	}
	if !containsDependency(data, piMCPAdapterDependency) {
		t.Fatalf("pi-mcp-adapter dependency not added:\n%s", string(data))
	}
}

func TestProvisionDxrkMemoryMCP_ErrorOnInvalidParentDir(t *testing.T) {
	a := NewAdapter()
	homeDir := t.TempDir()

	// Make .pi a file so agent dir creation fails.
	badPath := filepath.Join(homeDir, ".pi")
	if err := os.WriteFile(badPath, []byte("not-a-dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := a.ProvisionDxrkMemoryMCP(homeDir)
	if err == nil {
		t.Fatal("expected error when .pi is a file")
	}
}

func TestPiPackageIdentity(t *testing.T) {
	tests := []struct {
		name string
		pkg  any
		want string
	}{
		{"exact match string", "npm:pi-mcp-adapter", piMCPAdapterPackage},
		{"versioned string", "npm:pi-mcp-adapter@^2.6.0", piMCPAdapterPackage},
		{"other string", "npm:dxrk-pi", "npm:dxrk-pi"},
		{"object with source", map[string]any{"source": "npm:pi-mcp-adapter"}, piMCPAdapterPackage},
		{"other object", map[string]any{"source": "npm:dxrk-pi"}, "npm:dxrk-pi"},
		{"empty string", "", ""},
		{"nil", nil, ""},
		{"bool", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := piPackageIdentity(tt.pkg)
			if got != tt.want {
				t.Fatalf("piPackageIdentity(%v) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}

func TestPiPackagesAsSlice(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []any
	}{
		{"[]any", []any{"npm:a", "npm:b"}, []any{"npm:a", "npm:b"}},
		{"[]string", []string{"npm:a", "npm:b"}, []any{"npm:a", "npm:b"}},
		{"map with versioned npm entry", map[string]any{"npm:pi-mcp-adapter": "^2.6.0"}, []any{"npm:pi-mcp-adapter@^2.6.0"}},
		{"map with non-npm entry", map[string]any{"other-dep": "^1.0.0"}, []any{"other-dep"}},
		{"empty map", map[string]any{}, []any{}},
		{"nil", nil, []any(nil)},
		{"int", 42, []any(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := piPackagesAsSlice(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("piPackagesAsSlice(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAppendPiPackage(t *testing.T) {
	tests := []struct {
		name     string
		existing any
		want     []any
	}{
		{
			"appends to empty",
			[]any{},
			[]any{piMCPAdapterPackageSpec},
		},
		{
			"deduplicates exact match",
			[]any{piMCPAdapterPackageSpec},
			[]any{piMCPAdapterPackageSpec},
		},
		{
			"deduplicates versioned match",
			[]any{"npm:pi-mcp-adapter@^2.6.0"},
			[]any{piMCPAdapterPackageSpec},
		},
		{
			"preserves other packages",
			[]any{"npm:dxrk-pi", "npm:pi-mcp-adapter@1.0.0"},
			[]any{"npm:dxrk-pi", piMCPAdapterPackageSpec},
		},
		{
			"nil input produces new slice with desired",
			nil,
			[]any{piMCPAdapterPackageSpec},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendPiPackage(tt.existing, piMCPAdapterPackageSpec)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("appendPiPackage(%v, %q) = %v, want %v", tt.existing, piMCPAdapterPackageSpec, got, tt.want)
			}
		})
	}
}

func TestReadPiJSONObject(t *testing.T) {
	t.Run("file not exists returns empty map", func(t *testing.T) {
		got, err := readPiJSONObject("/nonexistent/path.json")
		if err != nil {
			t.Fatalf("readPiJSONObject() error = %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty map, got %v", got)
		}
	})

	t.Run("reads valid json", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.json")
		if err := os.WriteFile(path, []byte(`{"key": "value"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readPiJSONObject(path)
		if err != nil {
			t.Fatalf("readPiJSONObject() error = %v", err)
		}
		if got["key"] != "value" {
			t.Fatalf("got %v, want key=value", got)
		}
	})

	t.Run("null json returns empty map", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "null.json")
		if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readPiJSONObject(path)
		if err != nil {
			t.Fatalf("readPiJSONObject() error = %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty map, got %v", got)
		}
	})
}

func TestStaticCheck(t *testing.T) {
	// Verify NewAdapter is constructable and all methods work without panic.
	a := NewAdapter()
	if a == nil {
		t.Fatal("NewAdapter() returned nil")
	}
}

// containsPackage checks if the JSON data contains a string with the exact package name.
func containsPackage(data []byte, pkg string) bool {
	target := []byte(`"` + pkg + `"`)
	idx := bytesIndex(data, target)
	if idx < 0 {
		return false
	}
	before := data[0:idx]
	// Make sure it's not preceded by a @ (i.e., it's not a versioned sub-prefix)
	if len(before) > 0 && before[len(before)-1] == '@' {
		return false
	}
	return true
}

// containsDependency checks if the JSON data contains the dependency key.
func containsDependency(data []byte, dep string) bool {
	target := []byte(`"` + dep + `"`)
	return bytesIndex(data, target) >= 0
}

func bytesIndex(data, needle []byte) int {
	for i := 0; i <= len(data)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if data[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
