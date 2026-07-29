// SPDX-License-Identifier: MIT
package update

import (
	"fmt"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

const (
	dxrkName             = "dxrk"
	dxrkProgrammingOwner = "Dxrk777"
	dxrkMemoryName       = "dxrk-memory"
	dxrkGuardianName     = "dxrk-guardian"
	osDarwin             = "darwin"
	osLinux              = "linux"
	osWindows            = "windows"
)

// updateHint returns a platform-specific instruction string for updating the given tool.
func updateHint(tool ToolInfo, profile system.PlatformProfile) string {
	switch tool.Name {
	case dxrkName:
		return dxrkHint(profile)
	case dxrkMemoryName:
		return dxrkMemoryHint(profile)
	case dxrkGuardianName:
		return dxrkGuardianHint(profile)
	default:
		return ""
	}
}

func openCodeRegisteredNotMaterializedHint(tool ToolInfo) string {
	pkg := strings.TrimSpace(tool.NpmPackage)
	if pkg == "" {
		pkg = tool.Name
	}
	return fmt.Sprintf("registered in ~/.config/opencode/tui.json; pending npm dependency materialization for %s. Run dxrk upgrade to install/update ~/.config/opencode dependencies, then restart or reload OpenCode; if it stays pending, check OpenCode logs for package or peer dependency errors.", pkg)
}

func dxrkHint(profile system.PlatformProfile) string {
	switch profile.OS {
	case osDarwin:
		return "brew upgrade dxrk"
	case osLinux:
		return "curl -fsSL https://raw.githubusercontent.com/Dxrk777/Dxrk/main/scripts/install.sh | bash"
	case osWindows:
		return "irm https://raw.githubusercontent.com/Dxrk777/Dxrk/main/scripts/install.ps1 | iex"
	default:
		return ""
	}
}

func dxrkMemoryHint(profile system.PlatformProfile) string {
	switch profile.OS {
	case osDarwin:
		return "brew upgrade dxrk-memory"
	default:
		return "dxrk upgrade (downloads pre-built binary)"
	}
}

func dxrkGuardianHint(profile system.PlatformProfile) string {
	switch profile.OS {
	case osDarwin:
		return "brew upgrade dxrk-guardian"
	case osLinux:
		return "See https://github.com/Dxrk777/dxrk-guardian-angel"
	default:
		return ""
	}
}
