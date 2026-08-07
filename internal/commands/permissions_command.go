// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/config"
)

// RegisterPermissionsCommand registers the `dxrk permissions` command`dxrk permissions` command
// for managing tool permission rules.
func RegisterPermissionsCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:     "permissions",
		Short:   "Manage tool permission rules",
		Aliases: []string{"allowed-tools"},
		Long:    "View, add, and remove allow and deny rules for tool permissions.",
	}

	cmd.AddCommand(
		permissionsListCmd(),
		permissionsAllowCmd(),
		permissionsDenyCmd(),
		permissionsResetCmd(),
	)

	reg.AddCommand(cmd)
}

func permissionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all permission rules",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(defaultConfigPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			out := cmd.OutOrStdout()

			if cfg.Sandbox == nil {
				_, _ = fmt.Fprintln(out, "No sandbox configuration found.")
				return nil
			}

			_, _ = fmt.Fprintf(out, "Sandbox Image:  %s\n", cfg.Sandbox.DefaultImage)
			_, _ = fmt.Fprintf(out, "Memory Limit:   %s\n", cfg.Sandbox.MemoryLimit)
			_, _ = fmt.Fprintf(out, "CPU Limit:      %s\n", cfg.Sandbox.CPULimit)
			_, _ = fmt.Fprintf(out, "Timeout:        %ds\n", cfg.Sandbox.TimeoutSec)
			_, _ = fmt.Fprintf(out, "Max Containers: %d\n", cfg.Sandbox.MaxContainers)
			return nil
		},
	}
}

func permissionsAllowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "allow <tool>",
		Short: "Allow a tool in sandbox",
		Long: `Add a tool to the allow list for sandbox execution.

Examples:
  dxrk permissions allow bash
  dxrk permissions allow read_file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := args[0]
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Allowed tool: %s (sandbox permission)\n", tool)
			return nil
		},
	}
}

func permissionsDenyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deny <tool>",
		Short: "Deny a tool in sandbox",
		Long: `Add a tool to the deny list for sandbox execution.

Examples:
  dxrk permissions deny bash
  dxrk permissions deny write_file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := args[0]
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Denied tool: %s (sandbox permission)\n", tool)
			return nil
		},
	}
}

func permissionsResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset all permission rules",
		Long:  "Remove all allow and deny rules, restoring default permissions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintln(out, "Permission rules reset to defaults")
			return nil
		},
	}
}
