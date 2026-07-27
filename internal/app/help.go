// SPDX-License-Identifier: MIT
package app

import (
	"fmt"
	"io"
)

func printHelp(w io.Writer, version string) {
	_, _ = fmt.Fprintf(w, `dxrk — Dxrk: Ecosystem, Frameworks, Workflows (%s)

USAGE
  dxrk                     Launch interactive TUI
  dxrk <command> [flags]

COMMANDS
  install      Configure AI coding agents on this machine
  uninstall    Remove Dxrk AI managed files from this machine
  sync         Sync agent configs and skills to current version
  skill-registry refresh
               Refresh .atl/skill-registry.md with cache-hit fast path
  update       Check for available updates
  upgrade      Apply updates to managed tools
  restore      Restore a config backup
  version      Print version

FLAGS
  --help, -h    Show this help

Run 'dxrk help' for this message.
Documentation: https://github.com/Dxrk777/Dxrk
`, version)
}
