// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/config"
)

var validThemes = []string{"default", "dark", "light", "solarized", "monokai", "dracula", "nord"}

// RegisterThemeCommand registers the `dxrk theme` command for changing`dxrk theme` command for changing
// the UI theme.
func RegisterThemeCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "theme [name]",
		Short: "Change the UI theme",
		Long: `Change the terminal UI theme.

If no theme name is provided, shows available themes.

Examples:
  dxrk theme
  dxrk theme dark
  dxrk theme nord`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(defaultConfigPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			out := cmd.OutOrStdout()

			if len(args) == 0 {
				current := ""
				if cfg.WebUI != nil {
					current = cfg.WebUI.Theme
				}
				_, _ = fmt.Fprintf(out, "Current theme: %s\n", current)
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, "Available themes:")
				for _, t := range validThemes {
					marker := "  "
					if t == current {
						marker = "* "
					}
					_, _ = fmt.Fprintf(out, "%s%s\n", marker, t)
				}
				return nil
			}

			theme := strings.ToLower(strings.TrimSpace(args[0]))

			valid := false
			for _, t := range validThemes {
				if t == theme {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("unknown theme: %s. Available: %s",
					theme, strings.Join(validThemes, ", "))
			}

			if cfg.WebUI == nil {
				cfg.WebUI = &config.WebUIConfig{}
			}
			cfg.WebUI.Theme = theme

			if err := config.Save(defaultConfigPath(), cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(out, "Theme set to %s\n", theme)
			return nil
		},
	}

	reg.AddCommand(cmd)
}
