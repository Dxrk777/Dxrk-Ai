// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/config"
	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// RegisterModelCommand registers the `dxrk model` command for selecting`dxrk model` command for selecting
// the AI model.
func RegisterModelCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "model [name]",
		Short: "Set or view the AI model",
		Long: `Set the AI model for the current project or view available models.

If no model name is provided, shows the current model and available options.

Examples:
  dxrk model
  dxrk model claude-sonnet-4-20250514
  dxrk model claude-opus-4-20250918`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(defaultConfigPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if len(args) == 0 {
				out := cmd.OutOrStdout()
				_, _ = fmt.Fprintf(out, "Default provider: %s\n", cfg.Project.DefaultProvider)
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, "Configured providers:")
				for _, p := range cfg.Providers {
					marker := "  "
					if p.Name == cfg.Project.DefaultProvider {
						marker = "* "
					}
					_, _ = fmt.Fprintf(out, "%s%s: model=%s\n", marker, p.Name, p.Model)
				}
				return nil
			}

			model := strings.TrimSpace(args[0])
			if model == "default" {
				if len(cfg.Providers) > 0 {
					model = cfg.Providers[0].Model
				} else {
					model = strconst.StrClaudeSonnet420250514
				}
			}

			// Update the first provider's model or add a new one
			found := false
			for i, p := range cfg.Providers {
				if p.Name == cfg.Project.DefaultProvider {
					cfg.Providers[i].Model = model
					found = true
					break
				}
			}
			if !found && len(cfg.Providers) > 0 {
				cfg.Providers[0].Model = model
			}

			if err := config.Save(defaultConfigPath(), cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Model set to %s\n", model)
			return nil
		},
	}

	reg.AddCommand(cmd)
}
