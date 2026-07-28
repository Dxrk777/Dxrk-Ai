// SPDX-License-Identifier: MIT
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

// --- TestDetectInstalledVersion ---

func TestDetectInstalledVersion(t *testing.T) {
	tests := []struct {
		name          string
		tool          ToolInfo
		currentBuild  string
		lookPathFn    func(string) (string, error)
		execCommandFn func(string, ...string) *exec.Cmd
		wantVersion   string
	}{
		{
			name:         "dxrk uses build var",
			tool:         ToolInfo{Name: "dxrk", DetectCmd: nil},
			currentBuild: "1.5.0",
			wantVersion:  "1.5.0",
		},
		{
			name:         "dxrk dev build",
			tool:         ToolInfo{Name: "dxrk", DetectCmd: nil},
			currentBuild: "dev",
			wantVersion:  "dev",
		},
		{
			name: "dxrk-memory version parsed from output",
			tool: ToolInfo{Name: "dxrk-memory", DetectCmd: []string{"dxrk-memory", "version"}},
			lookPathFn: func(string) (string, error) {
				return "/usr/local/bin/dxrk-memory", nil
			},
			execCommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", "dxrk-memory v0.3.2")
			},
			wantVersion: "0.3.2",
		},
		{
			name: "dxrk-memory dev output is preserved as dev sentinel",
			tool: ToolInfo{Name: "dxrk-memory", DetectCmd: []string{"dxrk-memory", "version"}},
			lookPathFn: func(string) (string, error) {
				return "/usr/local/bin/dxrk-memory", nil
			},
			execCommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", "dxrk-memory dev")
			},
			wantVersion: "dev",
		},
		{
			name: "dxrk-guardian not installed",
			tool: ToolInfo{Name: "dxrk-guardian", DetectCmd: []string{"dxrk-guardian", "--version"}},
			lookPathFn: func(string) (string, error) {
				return "", fmt.Errorf("not found")
			},
			wantVersion: "",
		},
		{
			name: "binary exists but version command fails",
			tool: ToolInfo{Name: "dxrk-memory", DetectCmd: []string{"dxrk-memory", "version"}},
			lookPathFn: func(string) (string, error) {
				return "/usr/local/bin/dxrk-memory", nil
			},
			execCommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("false") // exits with error
			},
			wantVersion: "",
		},
		{
			name: "unparseable version output",
			tool: ToolInfo{Name: "dxrk-guardian", DetectCmd: []string{"dxrk-guardian", "--version"}},
			lookPathFn: func(string) (string, error) {
				return "/usr/local/bin/dxrk-guardian", nil
			},
			execCommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", "dxrk-guardian - no version info")
			},
			wantVersion: "",
		},
		{
			name:        "empty detect cmd slice",
			tool:        ToolInfo{Name: "test", DetectCmd: []string{}},
			wantVersion: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origLookPath := lookPath
			origExecCommand := execCommand
			t.Cleanup(func() {
				lookPath = origLookPath
				execCommand = origExecCommand
			})

			if tc.lookPathFn != nil {
				lookPath = tc.lookPathFn
			}
			if tc.execCommandFn != nil {
				execCommand = tc.execCommandFn
			}

			got := detectInstalledVersion(context.Background(), tc.tool, tc.currentBuild)
			if got != tc.wantVersion {
				t.Fatalf("detectInstalledVersion() = %q, want %q", got, tc.wantVersion)
			}
		})
	}
}

func TestDetectInstalledVersionFromOpenCodeNodeModulePackageJSON(t *testing.T) {
	home := t.TempDir()
	pkgDir := filepath.Join(home, ".config", "opencode", "node_modules", "opencode-sdd-dxrk-memory-manage")
	if err := os.MkdirAll(pkgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"version":"1.1.7"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	origHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = origHome })

	tool := ToolInfo{Name: "sdd-dxrk-memory-plugin", NpmPackage: "opencode-sdd-dxrk-memory-manage"}
	if got := detectInstalledVersion(context.Background(), tool, "dev"); got != "1.1.7" {
		t.Fatalf("detectInstalledVersion() = %q, want 1.1.7", got)
	}
}

func TestDetectInstalledVersionFromOpenCodePackageJSONDependency(t *testing.T) {
	home := t.TempDir()
	opencodeDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(opencodeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opencodeDir, "package.json"), []byte(`{"dependencies":{"opencode-sdd-dxrk-memory-manage":"^1.3.3"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	origHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = origHome })

	tool := ToolInfo{Name: "sdd-dxrk-memory-plugin", NpmPackage: "opencode-sdd-dxrk-memory-manage"}
	if got := detectInstalledVersion(context.Background(), tool, "dev"); got != "1.3.3" {
		t.Fatalf("detectInstalledVersion() = %q, want 1.3.3", got)
	}
}

func TestCheckSingleToolOpenCodePluginRegisteredNotMaterialized(t *testing.T) {
	home := t.TempDir()
	opencodeDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(opencodeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opencodeDir, "tui.json"), []byte(`{"plugin":["opencode-sdd-dxrk-memory-manage"]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	origHome := userHomeDir
	origClient := httpClient
	t.Cleanup(func() {
		userHomeDir = origHome
		httpClient = origClient
	})
	userHomeDir = func() (string, error) { return home, nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.2.3", HTMLURL: "https://example.test/release"})
	}))
	defer server.Close()
	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	tool := ToolInfo{
		Name:          "opencode-sdd-dxrk-memory-manage",
		Owner:         "owner",
		Repo:          "repo",
		InstallMethod: InstallOpenCodePlugin,
		NpmPackage:    "opencode-sdd-dxrk-memory-manage",
	}

	result := checkSingleTool(context.Background(), tool, "dev", system.PlatformProfile{})
	if result.Status != RegisteredNotMaterialized {
		t.Fatalf("status = %q, want %q", result.Status, RegisteredNotMaterialized)
	}
	if result.InstalledVersion != "" {
		t.Fatalf("InstalledVersion = %q, want empty while package.json is missing", result.InstalledVersion)
	}
	if !strings.Contains(strings.ToLower(result.UpdateHint), "restart or reload opencode") {
		t.Fatalf("UpdateHint should tell the user to restart/reload OpenCode, got %q", result.UpdateHint)
	}
	if !strings.Contains(result.UpdateHint, "peer dependency") {
		t.Fatalf("UpdateHint should mention checking logs for dependency errors, got %q", result.UpdateHint)
	}
}

func TestParseVersionFromOutput_DevSentinel(t *testing.T) {
	if got := parseVersionFromOutput("dxrk-memory dev"); got != "dev" {
		t.Fatalf("parseVersionFromOutput(dxrk-memory dev) = %q, want %q", got, "dev")
	}
}

// --- TestFetchLatestRelease ---

func TestFetchLatestRelease(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    interface{}
		wantTag string
		wantURL string
		wantErr bool
	}{
		{
			name:   "success 200",
			status: http.StatusOK,
			body: githubRelease{
				TagName: "v1.2.3",
				HTMLURL: "https://github.com/owner/repo/releases/tag/v1.2.3",
			},
			wantTag: "v1.2.3",
			wantURL: "https://github.com/owner/repo/releases/tag/v1.2.3",
		},
		{
			name:    "rate limit 403",
			status:  http.StatusForbidden,
			body:    map[string]string{"message": "rate limit exceeded"},
			wantErr: true,
		},
		{
			name:    "not found 404",
			status:  http.StatusNotFound,
			body:    map[string]string{"message": "Not Found"},
			wantErr: true,
		},
		{
			name:    "server error 500",
			status:  http.StatusInternalServerError,
			body:    map[string]string{"message": "Internal Server Error"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(tc.body)
			}))
			defer server.Close()

			origClient := httpClient
			t.Cleanup(func() { httpClient = origClient })

			// Override the HTTP client to point at the test server.
			// We also need to override the URL construction, so we use a custom transport.
			httpClient = server.Client()

			// We can't easily override the URL in fetchLatestRelease, so let's test
			// via a helper that accepts a base URL. Instead, we'll use a roundTripper
			// that redirects requests to our test server.
			httpClient.Transport = &testTransport{server: server}

			release, err := fetchLatestRelease(context.Background(), "owner", "repo")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if release.TagName != tc.wantTag {
				t.Fatalf("TagName = %q, want %q", release.TagName, tc.wantTag)
			}

			if release.HTMLURL != tc.wantURL {
				t.Fatalf("HTMLURL = %q, want %q", release.HTMLURL, tc.wantURL)
			}
		})
	}
}

// TestFetchLatestRelease_Timeout verifies timeout handling.
func TestFetchLatestRelease_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until context is cancelled — simulates a slow server.
		<-r.Context().Done()
	}))
	defer server.Close()

	origClient := httpClient
	t.Cleanup(func() { httpClient = origClient })

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately to force timeout

	_, err := fetchLatestRelease(ctx, "owner", "repo")
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}

// TestFetchLatestRelease_GithubToken verifies that GITHUB_TOKEN is sent as Bearer.
func TestFetchLatestRelease_GithubToken(t *testing.T) {
	var gotAuth string
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
	}))
	defer server.Close()

	origClient := httpClient
	t.Cleanup(func() { httpClient = origClient })

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	t.Setenv("GITHUB_TOKEN", "  test-token-123  ")

	_, err := fetchLatestRelease(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer test-token-123" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer test-token-123")
	}

	if gotUserAgent != "dxrk-update-check" {
		t.Fatalf("User-Agent = %q, want %q", gotUserAgent, "dxrk-update-check")
	}
}

// TestResolveGitHubToken_EnvVarWins verifies GITHUB_TOKEN takes precedence over gh CLI.
func TestResolveGitHubToken_EnvVarWins(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	if got := resolveGitHubToken(); got != "env-token" {
		t.Fatalf("resolveGitHubToken() = %q, want %q", got, "env-token")
	}
}

// TestResolveGitHubToken_GHTokenFallback verifies GH_TOKEN is used when
// GITHUB_TOKEN is unset, matching the gh CLI environment convention.
func TestResolveGitHubToken_GHTokenFallback(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", " gh-token ")

	if got := resolveGitHubToken(); got != "gh-token" {
		t.Fatalf("resolveGitHubToken() = %q, want %q", got, "gh-token")
	}
}

// TestResolveGitHubToken_EmptyWhenNoEnvAndNoGh verifies empty string returned when
// GITHUB_TOKEN is unset and gh is not in PATH.
func TestResolveGitHubToken_EmptyWhenNoEnvAndNoGh(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	origLookPath := ghLookPath
	t.Cleanup(func() { ghLookPath = origLookPath })
	ghLookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }

	if got := resolveGitHubToken(); got != "" {
		t.Fatalf("resolveGitHubToken() = %q, want empty", got)
	}
}

// --- TestCheckAll ---

func TestCheckAll(t *testing.T) {
	// Set up fake GitHub API that returns different versions per repo.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		path := r.URL.Path
		var release githubRelease
		switch {
		case contains(path, "dxrk-guardian-angel"):
			release = githubRelease{TagName: "v2.0.0", HTMLURL: "https://github.com/Dxrk777/dxrk-guardian-angel/releases/tag/v2.0.0"}
		case contains(path, "sub-agent-statusline"):
			release = githubRelease{TagName: "v0.4.0", HTMLURL: "https://github.com/Joaquinvesapa/sub-agent-statusline/releases/tag/v0.4.0"}
		case contains(path, "sdd-dxrk-memory-plugin"):
			release = githubRelease{TagName: "v1.1.7", HTMLURL: "https://github.com/j0k3r-dev-rgl/sdd-dxrk-memory-plugin/releases/tag/v1.1.7"}
		case contains(path, "dxrk-memory"):
			release = githubRelease{TagName: "v0.4.0", HTMLURL: "https://github.com/Dxrk777/dxrk-memory/releases/tag/v0.4.0"}
		case contains(path, "dxrk"):
			release = githubRelease{TagName: "v1.5.0", HTMLURL: "https://github.com/Dxrk777/Dxrk-Ai/releases/tag/v1.5.0"}
		}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	origUserHomeDir := userHomeDir
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
		userHomeDir = origUserHomeDir
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	// Mock: dxrk-memory is installed at v0.3.2, dxrk-guardian is not installed.
	lookPath = func(name string) (string, error) {
		switch name {
		case "dxrk-memory":
			return "/usr/local/bin/dxrk-memory", nil
		case "dxrk-guardian":
			return "", fmt.Errorf("not found")
		default:
			return "", fmt.Errorf("not found")
		}
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "dxrk-memory" {
			return exec.Command("echo", "dxrk-memory v0.3.2")
		}
		return exec.Command("false")
	}
	pluginHome := t.TempDir()
	userHomeDir = func() (string, error) { return pluginHome, nil }

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}
	results := CheckAll(context.Background(), "1.5.0", profile)

	if len(results) != 5 {
		t.Fatalf("len(results) = %d, want 5", len(results))
	}

	// dxrk: 1.5.0 local == 1.5.0 remote → UpToDate
	assertResult(t, results[0], "dxrk", UpToDate, "1.5.0", "1.5.0")

	// dxrk-memory: 0.3.2 local < 0.4.0 remote → UpdateAvailable
	assertResult(t, results[1], "dxrk-memory", UpdateAvailable, "0.3.2", "0.4.0")

	// dxrk-guardian: not installed
	assertResult(t, results[2], "dxrk-guardian", NotInstalled, "", "2.0.0")
	assertResult(t, results[3], "opencode-subagent-statusline", NotInstalled, "", "0.4.0")
	assertResult(t, results[4], "opencode-sdd-dxrk-memory-manage", NotInstalled, "", "1.1.7")
}

func TestCheckAll_NetworkError(t *testing.T) {
	// Server that immediately closes connections.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close the connection without responding properly.
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("false") }

	profile := system.PlatformProfile{OS: "linux", LinuxDistro: "ubuntu", PackageManager: "apt", Supported: true}
	results := CheckAll(context.Background(), "1.0.0", profile)

	if len(results) != 5 {
		t.Fatalf("len(results) = %d, want 5", len(results))
	}

	// dxrk has no DetectCmd, so it gets currentBuildVersion "1.0.0" as local
	// but fetch fails → CheckFailed (it has a local version).
	if results[0].Status != CheckFailed {
		t.Fatalf("dxrk status = %q, want %q", results[0].Status, CheckFailed)
	}
	if results[0].Err == nil {
		t.Fatalf("dxrk expected error, got nil")
	}

	// All other tools should also have CheckFailed since the server closes connections.
	for i, name := range []string{"dxrk-memory", "dxrk-guardian", "opencode-subagent-statusline", "opencode-sdd-dxrk-memory-manage"} {
		if results[i+1].Status != CheckFailed {
			t.Fatalf("%s status = %q, want %q", name, results[i+1].Status, CheckFailed)
		}
	}
}

func TestCheckFiltered_FetchErrorPreservesCheckFailedForMissingTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}
	lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("false") }

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}
	results := CheckFiltered(context.Background(), "1.0.0", profile, []string{"dxrk-memory"})

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Status != CheckFailed {
		t.Fatalf("dxrk-memory status = %q, want %q", results[0].Status, CheckFailed)
	}
}

// --- TestUpdateHint ---

func TestUpdateHint(t *testing.T) {
	tests := []struct {
		name    string
		tool    ToolInfo
		profile system.PlatformProfile
		want    string
	}{
		{
			name:    "dxrk macOS",
			tool:    ToolInfo{Name: "dxrk"},
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    "brew upgrade dxrk",
		},
		{
			name:    "dxrk linux",
			tool:    ToolInfo{Name: "dxrk"},
			profile: system.PlatformProfile{OS: "linux", PackageManager: "apt"},
			want:    "curl -fsSL https://raw.githubusercontent.com/Dxrk777/Dxrk/main/scripts/install.sh | bash",
		},
		{
			name:    "dxrk windows",
			tool:    ToolInfo{Name: "dxrk"},
			profile: system.PlatformProfile{OS: "windows", PackageManager: "winget"},
			want:    "irm https://raw.githubusercontent.com/Dxrk777/Dxrk/main/scripts/install.ps1 | iex",
		},
		{
			name:    "dxrk-memory macOS brew",
			tool:    ToolInfo{Name: "dxrk-memory"},
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    "brew upgrade dxrk-memory",
		},
		{
			name:    "dxrk-memory linux",
			tool:    ToolInfo{Name: "dxrk-memory"},
			profile: system.PlatformProfile{OS: "linux", PackageManager: "apt"},
			want:    "dxrk upgrade (downloads pre-built binary)",
		},
		{
			name:    "dxrk-memory windows",
			tool:    ToolInfo{Name: "dxrk-memory"},
			profile: system.PlatformProfile{OS: "windows", PackageManager: "winget"},
			want:    "dxrk upgrade (downloads pre-built binary)",
		},
		{
			name:    "dxrk-guardian macOS brew",
			tool:    ToolInfo{Name: "dxrk-guardian"},
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    "brew upgrade dxrk-guardian",
		},
		{
			name:    "dxrk-guardian linux",
			tool:    ToolInfo{Name: "dxrk-guardian"},
			profile: system.PlatformProfile{OS: "linux", PackageManager: "apt"},
			want:    "See https://github.com/Dxrk777/dxrk-guardian-angel",
		},
		{
			name:    "unknown tool",
			tool:    ToolInfo{Name: "unknown"},
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := updateHint(tc.tool, tc.profile)
			if got != tc.want {
				t.Fatalf("updateHint(%q, %q) = %q, want %q", tc.tool.Name, tc.profile.OS, got, tc.want)
			}
		})
	}
}

// --- TestVersionComparison ---

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		name   string
		local  string
		remote string
		want   UpdateStatus
	}{
		{name: "equal", local: "1.2.3", remote: "1.2.3", want: UpToDate},
		{name: "local newer major", local: "2.0.0", remote: "1.9.9", want: UpToDate},
		{name: "local newer minor", local: "1.3.0", remote: "1.2.9", want: UpToDate},
		{name: "local newer patch", local: "1.2.4", remote: "1.2.3", want: UpToDate},
		{name: "remote newer major", local: "1.0.0", remote: "2.0.0", want: UpdateAvailable},
		{name: "remote newer minor", local: "1.2.0", remote: "1.3.0", want: UpdateAvailable},
		{name: "remote newer patch", local: "1.2.3", remote: "1.2.4", want: UpdateAvailable},
		{name: "missing patch local", local: "1.2", remote: "1.2.1", want: UpdateAvailable},
		{name: "missing patch remote", local: "1.2.1", remote: "1.2", want: UpToDate},
		{name: "both missing patch equal", local: "1.2", remote: "1.2", want: UpToDate},
		{name: "zeros", local: "0.0.0", remote: "0.0.0", want: UpToDate},
		{name: "zero vs nonzero", local: "0.0.0", remote: "0.0.1", want: UpdateAvailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compareVersions(tc.local, tc.remote)
			if got != tc.want {
				t.Fatalf("compareVersions(%q, %q) = %q, want %q", tc.local, tc.remote, got, tc.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "with v prefix", raw: "v1.2.3", want: "1.2.3"},
		{name: "without prefix", raw: "1.2.3", want: "1.2.3"},
		{name: "with spaces", raw: "  v1.2.3  ", want: "1.2.3"},
		{name: "two parts", raw: "v1.2", want: "1.2"},
		{name: "dev", raw: "dev", want: "dev"},
		{name: "empty", raw: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeVersion(tc.raw)
			if got != tc.want {
				t.Fatalf("normalizeVersion(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestIsSemver(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"1.2.3", true},
		{"1.2", true},
		{"0.0.0", true},
		{"dev", false},
		{"", false},
		{"abc", false},
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			got := isSemver(tc.version)
			if got != tc.want {
				t.Fatalf("isSemver(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestParseVersionParts(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    [3]int
	}{
		{name: "full semver", version: "1.2.3", want: [3]int{1, 2, 3}},
		{name: "two parts", version: "1.2", want: [3]int{1, 2, 0}},
		{name: "one part", version: "1", want: [3]int{1, 0, 0}},
		{name: "empty", version: "", want: [3]int{0, 0, 0}},
		{name: "non-numeric", version: "abc.def", want: [3]int{0, 0, 0}},
		{name: "large numbers", version: "100.200.300", want: [3]int{100, 200, 300}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseVersionParts(tc.version)
			if got != tc.want {
				t.Fatalf("parseVersionParts(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestParseVersionFromOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{name: "dxrk-memory v0.3.2", output: "dxrk-memory v0.3.2", want: "0.3.2"},
		{name: "dxrk-guardian 1.0.0", output: "dxrk-guardian version 1.0.0", want: "1.0.0"},
		{name: "bare version", output: "2.1.0", want: "2.1.0"},
		{name: "no version", output: "no version info here", want: ""},
		{name: "empty", output: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseVersionFromOutput(tc.output)
			if got != tc.want {
				t.Fatalf("parseVersionFromOutput(%q) = %q, want %q", tc.output, got, tc.want)
			}
		})
	}
}

// TestRegistryContents verifies the registry has all expected tools.
func TestRegistryContents(t *testing.T) {
	if len(Tools) != 5 {
		t.Fatalf("len(Tools) = %d, want 5", len(Tools))
	}

	expected := map[string]struct {
		owner string
		repo  string
	}{
		"dxrk":                            {owner: "Dxrk777", repo: "dxrk"},
		"dxrk-memory":                     {owner: "Dxrk777", repo: "dxrk-memory"},
		"dxrk-guardian":                   {owner: "Dxrk777", repo: "dxrk-guardian-angel"},
		"opencode-subagent-statusline":    {owner: "Joaquinvesapa", repo: "sub-agent-statusline"},
		"opencode-sdd-dxrk-memory-manage": {owner: "j0k3r-dev-rgl", repo: "sdd-dxrk-memory-plugin"},
	}

	for _, tool := range Tools {
		exp, ok := expected[tool.Name]
		if !ok {
			t.Fatalf("unexpected tool in registry: %q", tool.Name)
		}
		if tool.Owner != exp.owner {
			t.Fatalf("tool %q Owner = %q, want %q", tool.Name, tool.Owner, exp.owner)
		}
		if tool.Repo != exp.repo {
			t.Fatalf("tool %q Repo = %q, want %q", tool.Name, tool.Repo, exp.repo)
		}
	}

	// dxrk must have nil DetectCmd.
	if Tools[0].DetectCmd != nil {
		t.Fatalf("dxrk DetectCmd should be nil")
	}

	// dxrk-memory and dxrk-guardian must have non-nil DetectCmd.
	if Tools[1].DetectCmd == nil {
		t.Fatalf("dxrk-memory DetectCmd should not be nil")
	}
	if Tools[2].DetectCmd == nil {
		t.Fatalf("dxrk-guardian DetectCmd should not be nil")
	}
	if Tools[3].NpmPackage == "" || Tools[4].NpmPackage == "" {
		t.Fatalf("OpenCode plugin tools should declare NpmPackage")
	}
}

// TestCheckAll_DevVersion verifies that "dev" build version results in DevBuild
// (not VersionUnknown — dev is a well-known sentinel for source-built binaries).
func TestCheckAll_DevVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand

	// Override only the first tool (dxrk) by running CheckAll with "dev".
	origTools := Tools
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
		Tools = origTools
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	// Restrict to just dxrk to isolate the test.
	Tools = []ToolInfo{Tools[0]}

	lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("false") }

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}
	results := CheckAll(context.Background(), "dev", profile)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	// The spec requires: "dev" build MUST be reported as DevBuild, not VersionUnknown.
	if results[0].Status != DevBuild {
		t.Fatalf("dxrk dev status = %q, want %q", results[0].Status, DevBuild)
	}
}

// --- TestCheckFiltered ---

// TestCheckFiltered verifies that CheckFiltered restricts results to the named tools
// and that the dev-build sentinel causes dxrk to be reported as DevBuild.
func TestCheckFiltered_SubsetOfTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0", HTMLURL: "https://github.com/example/repo/releases/tag/v1.0.0"})
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}
	lookPath = func(name string) (string, error) {
		if name == "dxrk-memory" {
			return "/usr/local/bin/dxrk-memory", nil
		}
		return "", fmt.Errorf("not found")
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "dxrk-memory" {
			return exec.Command("echo", "dxrk-memory v0.9.9")
		}
		return exec.Command("false")
	}

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}

	// Request only "dxrk-memory" — should return exactly 1 result.
	results := CheckFiltered(context.Background(), "1.0.0", profile, []string{"dxrk-memory"})
	if len(results) != 1 {
		t.Fatalf("CheckFiltered(dxrk-memory) len = %d, want 1", len(results))
	}
	if results[0].Tool.Name != "dxrk-memory" {
		t.Fatalf("CheckFiltered(dxrk-memory) tool = %q, want %q", results[0].Tool.Name, "dxrk-memory")
	}
}

// TestCheckFiltered_EmptyFilter verifies that an empty filter returns all tools (same as CheckAll).
func TestCheckFiltered_EmptyFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}
	lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("false") }

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}

	// nil filter → all tools (same as CheckAll).
	results := CheckFiltered(context.Background(), "1.0.0", profile, nil)
	if len(results) != len(Tools) {
		t.Fatalf("CheckFiltered(nil) len = %d, want %d", len(results), len(Tools))
	}
}

// TestCheckFiltered_UnknownToolIgnored verifies that requesting an unknown tool name is
// silently skipped without panicking or returning garbage results.
func TestCheckFiltered_UnknownToolIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}
	lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("false") }

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}

	results := CheckFiltered(context.Background(), "1.0.0", profile, []string{"no-such-tool"})
	if len(results) != 0 {
		t.Fatalf("CheckFiltered(no-such-tool) len = %d, want 0", len(results))
	}
}

// TestCheckFiltered_DevBuildSemanticsForDxrkAI verifies the design requirement:
// when the running dxrk binary reports version "dev", it is identified as a
// DevBuild and NOT reported as UpdateAvailable or VersionUnknown.
//
// The spec says:
//   - Dev build MUST be reported as development-build semantic
//   - dxrk self-upgrade is skipped while dxrk-memory/dxrk-guardian remain eligible
func TestCheckFiltered_DevBuildSemanticsForDxrkAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v9.9.9"})
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	origTools := Tools
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
		Tools = origTools
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}
	lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	execCommand = func(name string, args ...string) *exec.Cmd { return exec.Command("false") }
	Tools = []ToolInfo{Tools[0]} // dxrk only

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}

	results := CheckFiltered(context.Background(), "dev", profile, nil)
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}

	r := results[0]
	if r.Tool.Name != "dxrk" {
		t.Fatalf("tool = %q, want dxrk", r.Tool.Name)
	}

	// Dev build should be reported as DevBuild status, not VersionUnknown or UpdateAvailable.
	if r.Status != DevBuild {
		t.Fatalf("dev status = %q, want DevBuild; ensure DevBuild status is used for dev version builds", r.Status)
	}
}

// TestCheckFiltered_DevBuildSkipNotEligible verifies that in a mixed run,
// dxrk with "dev" version gets DevBuild while dxrk-memory with a real version stays eligible.
func TestCheckFiltered_DevBuildSkipNotEligible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		path := r.URL.Path
		var release githubRelease
		switch {
		case contains(path, "dxrk"):
			release = githubRelease{TagName: "v9.9.9"}
		case contains(path, "dxrk-memory"):
			release = githubRelease{TagName: "v2.0.0"}
		default:
			release = githubRelease{TagName: "v1.0.0"}
		}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	origTools := Tools
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
		Tools = origTools
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	// dxrk-memory is installed at v1.0.0
	lookPath = func(name string) (string, error) {
		if name == "dxrk-memory" {
			return "/usr/local/bin/dxrk-memory", nil
		}
		return "", fmt.Errorf("not found")
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "dxrk-memory" {
			return exec.Command("echo", "dxrk-memory v1.0.0")
		}
		return exec.Command("false")
	}
	// Only dxrk and dxrk-memory for this test
	Tools = []ToolInfo{Tools[0], Tools[1]}

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}

	results := CheckFiltered(context.Background(), "dev", profile, nil)
	if len(results) != 2 {
		t.Fatalf("len = %d, want 2", len(results))
	}

	// dxrk should be DevBuild
	if results[0].Status != DevBuild {
		t.Fatalf("dxrk status = %q, want DevBuild", results[0].Status)
	}

	// dxrk-memory should be UpdateAvailable (1.0.0 < 2.0.0)
	if results[1].Status != UpdateAvailable {
		t.Fatalf("dxrk-memory status = %q, want UpdateAvailable", results[1].Status)
	}
}

// TestNoUpdatesPath verifies CheckFiltered returns correct statuses when nothing needs updating.
func TestNoUpdatesPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		path := r.URL.Path
		var release githubRelease
		switch {
		case contains(path, "dxrk-memory"):
			release = githubRelease{TagName: "v0.3.2"}
		case contains(path, "dxrk-guardian-angel"):
			release = githubRelease{TagName: "v1.0.0"}
		default:
			release = githubRelease{TagName: "v1.0.0"}
		}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	origTools := Tools
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
		Tools = origTools
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}

	// dxrk-memory is at v0.3.2 (same as remote), dxrk-guardian is not installed
	lookPath = func(name string) (string, error) {
		if name == "dxrk-memory" {
			return "/usr/local/bin/dxrk-memory", nil
		}
		return "", fmt.Errorf("not found")
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "dxrk-memory" {
			return exec.Command("echo", "dxrk-memory v0.3.2")
		}
		return exec.Command("false")
	}
	// Only dxrk-memory and dxrk-guardian for this test (skip dxrk to avoid dev-build behavior)
	Tools = []ToolInfo{Tools[1], Tools[2]}

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}

	results := CheckFiltered(context.Background(), "1.0.0", profile, nil)
	if len(results) != 2 {
		t.Fatalf("len = %d, want 2", len(results))
	}

	// dxrk-memory: up to date
	if results[0].Status != UpToDate {
		t.Fatalf("dxrk-memory status = %q, want UpToDate", results[0].Status)
	}

	// dxrk-guardian: not installed
	if results[1].Status != NotInstalled {
		t.Fatalf("dxrk-guardian status = %q, want NotInstalled", results[1].Status)
	}
}

// --- TestEngramHintNoBrew ---

// TestEngramHintNoBrew verifies that on non-brew platforms, dxrkMemoryHint
// no longer returns "go install..." — it should reflect binary download.
// This is the regression test for issue #160.
func TestEngramHintNoBrew(t *testing.T) {
	tests := []struct {
		name    string
		profile system.PlatformProfile
	}{
		{
			name:    "linux apt",
			profile: system.PlatformProfile{OS: "linux", PackageManager: "apt"},
		},
		{
			name:    "windows winget",
			profile: system.PlatformProfile{OS: "windows", PackageManager: "winget"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := ToolInfo{Name: "dxrk-memory"}
			got := updateHint(tool, tc.profile)

			// Must NOT contain "go install".
			if contains(got, "go install") {
				t.Errorf("dxrkMemoryHint for non-brew should NOT contain 'go install', got %q", got)
			}

			// Must NOT be empty (should have some actionable hint).
			if got == "" {
				t.Errorf("dxrkMemoryHint for non-brew should not be empty")
			}
		})
	}
}

// TestInstallMethodFieldsOnRegistry verifies that InstallMethod is set on all tools.
func TestInstallMethodFieldsOnRegistry(t *testing.T) {
	for _, tool := range Tools {
		if tool.InstallMethod == "" {
			t.Errorf("tool %q has empty InstallMethod — must be set", tool.Name)
		}
	}

	// dxrk-memory: uses binary download (not go-install) — GoImportPath must be empty.
	for _, tool := range Tools {
		if tool.Name == "dxrk-memory" {
			if tool.InstallMethod != InstallBinary {
				t.Errorf("dxrk-memory InstallMethod = %q, want %q", tool.InstallMethod, InstallBinary)
			}
			if tool.GoImportPath != "" {
				t.Errorf("dxrk-memory GoImportPath should be empty (binary download, not go-install), got %q", tool.GoImportPath)
			}
		}
	}
}

// --- helpers ---

// testTransport redirects all requests to the test server.
type testTransport struct {
	server *httptest.Server
}

func (tt *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to point at the test server, preserving the path.
	req.URL.Scheme = "http"
	req.URL.Host = tt.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

func assertResult(t *testing.T, r UpdateResult, wantName string, wantStatus UpdateStatus, wantInstalled, wantLatest string) {
	t.Helper()

	if r.Tool.Name != wantName {
		t.Fatalf("tool name = %q, want %q", r.Tool.Name, wantName)
	}
	if r.Status != wantStatus {
		t.Fatalf("%s status = %q, want %q (installed=%q, latest=%q, err=%v)",
			wantName, r.Status, wantStatus, r.InstalledVersion, r.LatestVersion, r.Err)
	}
	if r.InstalledVersion != wantInstalled {
		t.Fatalf("%s InstalledVersion = %q, want %q", wantName, r.InstalledVersion, wantInstalled)
	}
	if r.LatestVersion != wantLatest {
		t.Fatalf("%s LatestVersion = %q, want %q", wantName, r.LatestVersion, wantLatest)
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
