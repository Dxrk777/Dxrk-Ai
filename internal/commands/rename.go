// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// RegisterRenameCommand registers the `dxrk rename` command for renaming`dxrk rename` command for renaming
// the current or a specified session.
func RegisterRenameCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "rename <session-id> <new-name>",
		Short: "Rename a session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			newName := strings.TrimSpace(args[1])
			if newName == "" {
				return fmt.Errorf("new name cannot be empty")
			}

			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			for i := range sessions {
				if sessions[i].ID == sessionID || strings.HasPrefix(sessions[i].ID, sessionID) {
					s := sessions[i]
					oldTitle := s.Title
					s.Title = newName
					s.UpdatedAt = time.Now()

					if err := saveSession(&s); err != nil {
						return fmt.Errorf("save renamed session: %w", err)
					}

					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Renamed session %s: %q → %q\n",
						s.ID[:8], oldTitle, newName)
					return nil
				}
			}

			return fmt.Errorf("session %q not found", sessionID)
		},
	}

	reg.AddCommand(cmd)
}
