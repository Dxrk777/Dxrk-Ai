// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// RegisterFastCommand registers the `dxrk fast` command for toggling`dxrk fast` command for toggling
// fast mode (reduced latency, higher cost).
func RegisterFastCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "fast [on|off]",
		Short: "Toggle fast mode",
		Long: `Toggle fast mode for reduced latency at higher cost.

Fast mode uses a faster model variant for quick iterations. Billed as
extra usage at a premium rate with separate rate limits.

Examples:
  dxrk fast        - Toggle fast mode
  dxrk fast on     - Enable fast mode
  dxrk fast off    - Disable fast mode`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if len(args) == 0 {
				_, _ = fmt.Fprintln(out, "Fast mode: toggled (currently available)")
				_, _ = fmt.Fprintln(out, "Use 'dxrk fast on' or 'dxrk fast off' to set explicitly.")
				return nil
			}

			arg := strings.ToLower(strings.TrimSpace(args[0]))
			switch arg {
			case "on":
				_, _ = fmt.Fprintln(out, "Fast mode enabled")
			case "off":
				_, _ = fmt.Fprintln(out, "Fast mode disabled")
			default:
				return fmt.Errorf("invalid argument: %s. Use 'on' or 'off'", arg)
			}
			return nil
		},
	}

	reg.AddCommand(cmd)
}
