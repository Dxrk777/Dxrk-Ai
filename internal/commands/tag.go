// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/utils/session"
)

// RegisterTagCommand registers the `dxrk tag` command for adding, removing,`dxrk tag` command for adding, removing,
// and listing tags on sessions.
func RegisterTagCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage session tags",
		Long:  "Add, remove, or list tags on conversation sessions for organization.",
	}

	cmd.AddCommand(
		tagAddCmd(),
		tagRemoveCmd(),
		tagListCmd(),
		tagSearchCmd(),
	)

	reg.AddCommand(cmd)
}

func tagAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <session-id> <tag-name>",
		Short: "Add a tag to a session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			tagName := strings.TrimSpace(args[1])
			if tagName == "" {
				return fmt.Errorf("tag name cannot be empty")
			}

			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			for i := range sessions {
				if sessions[i].ID == sessionID || strings.HasPrefix(sessions[i].ID, sessionID) {
					s := sessions[i]

					// Check for duplicate tag.
					for _, t := range s.Tags {
						if t == tagName {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Tag %q already exists on session %s\n", tagName, s.ID[:8])
							return nil
						}
					}

					s.Tags = append(s.Tags, tagName)
					if err := saveSession(&s); err != nil {
						return fmt.Errorf("save tagged session: %w", err)
					}

					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added tag %q to session %s — %s\n",
						tagName, s.ID[:8], s.Title)
					return nil
				}
			}

			return fmt.Errorf("session %q not found", sessionID)
		},
	}
}

func tagRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <session-id> <tag-name>",
		Short: "Remove a tag from a session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			tagName := args[1]

			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			for i := range sessions {
				if sessions[i].ID == sessionID || strings.HasPrefix(sessions[i].ID, sessionID) {
					s := sessions[i]
					found := false
					filtered := make([]string, 0, len(s.Tags))
					for _, t := range s.Tags {
						if t == tagName {
							found = true
							continue
						}
						filtered = append(filtered, t)
					}

					if !found {
						return fmt.Errorf("tag %q not found on session %s", tagName, s.ID[:8])
					}

					s.Tags = filtered
					if err := saveSession(&s); err != nil {
						return fmt.Errorf("save session: %w", err)
					}

					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed tag %q from session %s\n", tagName, s.ID[:8])
					return nil
				}
			}

			return fmt.Errorf("session %q not found", sessionID)
		},
	}
}

func tagListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <session-id>",
		Short: "List tags on a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSession(args[0])
			if err != nil {
				return err
			}

			if len(s.Tags) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Session %s has no tags.\n", s.ID[:8])
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Tags for session %s (%s):\n", s.ID[:8], s.Title)
			for _, t := range s.Tags {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", t)
			}
			return nil
		},
	}
}

func tagSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <tag-name>",
		Short: "Find sessions with a specific tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tagName := args[0]

			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			var matches []session.Session
			for _, s := range sessions {
				for _, t := range s.Tags {
					if t == tagName {
						matches = append(matches, s)
						break
					}
				}
			}

			if len(matches) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No sessions found with tag %q\n", tagName)
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Sessions with tag %q (%d):\n\n", tagName, len(matches))
			for _, s := range matches {
				title := s.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-40s  %s\n",
					s.ID[:8], title, s.UpdatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
}
