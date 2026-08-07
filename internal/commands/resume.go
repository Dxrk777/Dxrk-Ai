// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/utils/session"
)

// RegisterResumeCommand registers the `dxrk resume` command for resuming`dxrk resume` command for resuming
// a previous session by ID or search term.
func RegisterResumeCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:     "resume [session-id-or-search]",
		Short:   "Resume a previous conversation",
		Aliases: []string{"continue"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions found. Create one with `dxrk session create`.")
				return nil
			}

			// No argument: show recent sessions for selection.
			if len(args) == 0 {
				return resumeListInteractive(sessions, cmd)
			}

			query := args[0]

			// Try exact/prefix match on ID first.
			for i := range sessions {
				if sessions[i].ID == query || strings.HasPrefix(sessions[i].ID, query) {
					return resumeSession(&sessions[i], cmd)
				}
			}

			// Try title substring match.
			queryLower := strings.ToLower(query)
			for i := range sessions {
				if strings.Contains(strings.ToLower(sessions[i].Title), queryLower) {
					return resumeSession(&sessions[i], cmd)
				}
			}

			// Try tag match.
			for i := range sessions {
				for _, t := range sessions[i].Tags {
					if strings.EqualFold(t, query) {
						return resumeSession(&sessions[i], cmd)
					}
				}
			}

			return fmt.Errorf("no session matching %q — use `dxrk resume` to list recent sessions", query)
		},
	}

	reg.AddCommand(cmd)
}

func resumeListInteractive(sessions []session.Session, cmd *cobra.Command) error {
	limit := 10
	if len(sessions) < limit {
		limit = len(sessions)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Recent sessions (showing %d of %d):\n\n", limit, len(sessions))
	for i := 0; i < limit; i++ {
		s := sessions[i]
		title := s.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		_, _ = fmt.Fprintf(out, "  %s  %-50s  %s  %d msgs\n",
			s.ID[:8],
			title,
			s.UpdatedAt.Format("2006-01-02 15:04"),
			s.MessageCount,
		)
	}
	_, _ = fmt.Fprintf(out, "\nResume with: dxrk resume <id-or-title>\n")
	return nil
}

func resumeSession(s *session.Session, cmd *cobra.Command) error {
	s.Status = session.Active
	if err := saveSession(s); err != nil {
		return fmt.Errorf("resume session: %w", err)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Resumed session %s — %s\n", s.ID[:8], s.Title)
	_, _ = fmt.Fprintf(out, "Model: %s | Messages: %d | Tokens: %d\n", s.Model, s.MessageCount, s.TokenCount)
	if len(s.Messages) > 0 {
		last := s.Messages[len(s.Messages)-1]
		content := last.Content
		if len(content) > 120 {
			content = content[:117] + "..."
		}
		_, _ = fmt.Fprintf(out, "Last message (%s): %s\n", last.Role, content)
	}
	return nil
}
