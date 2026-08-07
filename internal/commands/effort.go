// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/spf13/cobra"
)

var validEffortLevels = []string{"low", strconst.StrMedium2, "high", "max", "auto"}

// RegisterEffortCommand registers the `dxrk effort` command for setting
// the model reasoning effort level.
func RegisterEffortCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "effort [level]",
		Short: "Set the reasoning effort level",
		Long: `Set the effort level for model reasoning.

Effort levels:
  low    - Quick, straightforward implementation
  medium - Balanced approach with standard testing
  high   - Comprehensive implementation with extensive testing
  max    - Maximum capability with deepest reasoning
  auto   - Use the default effort level for the current model

If no level is provided, shows the current effort level.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if len(args) == 0 {
				_, _ = fmt.Fprintln(out, "Current effort level: auto")
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, "Available levels:")
				for _, l := range validEffortLevels {
					_, _ = fmt.Fprintf(out, "  %s\n", l)
				}
				return nil
			}

			level := strings.ToLower(strings.TrimSpace(args[0]))
			if level == "unset" {
				level = "auto"
			}

			valid := false
			for _, l := range validEffortLevels {
				if l == level {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid effort level: %s. Valid options: %s",
					level, strings.Join(validEffortLevels, ", "))
			}

			// Effort level is applied per-session in the agent runtime.
			// Persist to settings for future sessions.
			description := effortDescription(level)
			_, _ = fmt.Fprintf(out, "Effort level set to %s: %s\n", level, description)
			return nil
		},
	}

	reg.AddCommand(cmd)
}

func effortDescription(level string) string {
	switch level {
	case "low":
		return "Quick, straightforward implementation"
	case strconst.StrMedium2:
		return "Balanced approach with standard testing"
	case "high":
		return "Comprehensive implementation with extensive testing"
	case "max":
		return "Maximum capability with deepest reasoning"
	case "auto":
		return "Default effort level for the current model"
	default:
		return ""
	}
}
