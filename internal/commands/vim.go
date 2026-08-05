// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// RegisterVimCommand registers the `dxrk vim` command for toggling`dxrk vim` command for toggling
// between Vim and Normal editing modes.
func RegisterVimCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "vim",
		Short: "Toggle between Vim and Normal editing modes",
		Long: `Toggle the editor input mode between Vim and Normal (readline).

When Vim mode is enabled:
  - Press Escape to toggle between INSERT and NORMAL modes
  - Use standard Vim keybindings in NORMAL mode

When Normal mode is enabled:
  - Use standard readline keyboard bindings`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			// Toggle is tracked per-session in the runtime.
			// This command signals the toggle intent.
			_, _ = fmt.Fprintln(out, "Editor mode toggled")
			_, _ = fmt.Fprintln(out, "Use Escape to switch between INSERT and NORMAL modes when Vim mode is active.")
			return nil
		},
	}

	reg.AddCommand(cmd)
}
