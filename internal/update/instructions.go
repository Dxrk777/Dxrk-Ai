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
)

// updateHint returns a platform-specific instruction string for updating the given tool.
func updateHint(tool ToolInfo, profile system.PlatformProfile) string {
	switch tool.Name {
	case dxrkName:
		return dxrkHint(profile)
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
	case "darwin":
		return "brew upgrade dxrk"
	case "linux":
		return "curl -fsSL https://raw.githubusercontent.com/Dxrk777/Dxrk-Ai/main/scripts/install.sh | bash"
	case "windows":
		return "irm https://raw.githubusercontent.com/Dxrk777/Dxrk-Ai/main/scripts/install.ps1 | iex"
	default:
		return ""
	}
}
