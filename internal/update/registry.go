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
}
