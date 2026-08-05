// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// RegisterKeybindingsCommand registers the `dxrk keybindings` command`dxrk keybindings` command
// for managing custom keybindings.
func RegisterKeybindingsCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "keybindings",
		Short: "Open or create keybindings configuration",
		Long: `Open or create your custom keybindings configuration file.

Creates a default keybindings file if one does not exist, then opens
it in your editor for customization.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}

			dir := filepath.Join(home, ".dxrk")
			path := filepath.Join(dir, "keybindings.json")

			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create config directory: %w", err)
			}

			created := false
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if err := os.WriteFile(path, []byte(defaultKeybindingsJSON()), 0o644); err != nil {
					return fmt.Errorf("create keybindings file: %w", err)
				}
				created = true
			}

			if created {
				_, _ = fmt.Fprintf(out, "Created keybindings file: %s\n", path)
			} else {
				_, _ = fmt.Fprintf(out, "Keybindings file: %s\n", path)
			}
			return nil
		},
	}

	reg.AddCommand(cmd)
}

func defaultKeybindingsJSON() string {
	return `{
  "keybindings": [
    {
      "key": "ctrl+n",
      "action": "new_conversation",
      "description": "Start a new conversation"
    },
    {
      "key": "ctrl+s",
      "action": "save_session",
      "description": "Save current session"
    },
    {
      "key": "ctrl+r",
      "action": "resume_session",
      "description": "Resume last session"
    },
    {
      "key": "ctrl+l",
      "action": "clear_screen",
      "description": "Clear the terminal"
    }
  ]
}
`
}
