// SPDX-License-Identifier: MIT
package update

// Tools is the static registry of managed tools that can be checked for updates.
//
// InstallMethod controls which upgrade strategy the executor uses:
//   - InstallBrew: managed via homebrew (macOS/Linux with brew)
//   - InstallGoInstall: installed via `go install <GoImportPath>@version`
//   - InstallBinary: downloaded binary from GitHub Releases (atomic replace)
//
// For brew-managed platforms the executor picks brew regardless of the
// field here; InstallMethod represents the non-brew fallback strategy.
var Tools = []ToolInfo{
	{
		Name:          dxrkName,
		Owner:         dxrkProgrammingOwner,
		Repo:          dxrkName,
		DetectCmd:     nil,
		VersionPrefix: "v",
		InstallMethod: InstallBinary,
	},
	{
		Name:          dxrkMemoryName,
		Owner:         dxrkProgrammingOwner,
		Repo:          dxrkMemoryName,
		DetectCmd:     []string{"dxrk-memory"},
		VersionPrefix: "v",
		InstallMethod: InstallBinary,
	},
	{
		Name:          dxrkGuardianName,
		Owner:         dxrkProgrammingOwner,
		Repo:          "dxrk-guardian-angel",
		DetectCmd:     []string{"dxrk-guardian"},
		VersionPrefix: "v",
		InstallMethod: InstallScript,
	},
	{
		Name:          "opencode-subagent-statusline",
		Owner:         "Joaquinvesapa",
		Repo:          "sub-agent-statusline",
		DetectCmd:     nil,
		VersionPrefix: "v",
		InstallMethod: InstallOpenCodePlugin,
		NpmPackage:    "opencode-subagent-statusline",
	},
	{
		Name:          "opencode-sdd-dxrk-memory-manage",
		Owner:         "j0k3r-dev-rgl",
		Repo:          "sdd-dxrk-memory-plugin",
		DetectCmd:     nil,
		VersionPrefix: "v",
		InstallMethod: InstallOpenCodePlugin,
		NpmPackage:    "opencode-sdd-dxrk-memory-manage",
	},
}
