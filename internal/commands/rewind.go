// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk-Ai/internal/utils/session"
)

// RegisterRewindCommand registers the `dxrk rewind` command for reverting`dxrk rewind` command for reverting
// a session to a previous message point.
func RegisterRewindCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "rewind [session-id] [message-count]",
		Short: "Rewind session to a previous point",
		Long:  "Remove the last N message turns from a session, effectively rewinding the conversation.",
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var sessionID string
			turns := 1

			switch len(args) {
			case 2:
				sessionID = args[0]
				n, err := strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("invalid message count %q: %w", args[1], err)
				}
				turns = n
			case 1:
				// Could be session ID or message count.
				if n, err := strconv.Atoi(args[0]); err == nil {
					turns = n
				} else {
					sessionID = args[0]
				}
			}

			if turns < 1 {
				return fmt.Errorf("message count must be >= 1")
			}

			// Find session: use provided ID or latest active session.
			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found")
			}

			var target *session.Session
			if sessionID != "" {
				for i := range sessions {
					if sessions[i].ID == sessionID || strings.HasPrefix(sessions[i].ID, sessionID) {
						s := sessions[i]
						target = &s
						break
					}
				}
				if target == nil {
					return fmt.Errorf("session %q not found", sessionID)
				}
			} else {
				s := sessions[0]
				target = &s
			}

			return rewindSession(target, turns, cmd)
		},
	}

	reg.AddCommand(cmd)
}

func rewindSession(s *session.Session, turns int, cmd *cobra.Command) error {
	total := len(s.Messages)
	if total == 0 {
		return fmt.Errorf("session has no messages to rewind")
	}

	// Calculate how many messages to remove (each "turn" is a user+assistant pair).
	removeCount := turns * 2
	if removeCount > total {
		removeCount = total
	}

	removed := s.Messages[total-removeCount:]
	s.Messages = s.Messages[:total-removeCount]
	s.MessageCount = len(s.Messages)
	s.EstimateTokens()
	s.UpdatedAt = time.Now()

	if err := saveSession(s); err != nil {
		return fmt.Errorf("save rewound session: %w", err)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Rewound session %s by %d messages (%d remaining)\n",
		s.ID[:8], len(removed), len(s.Messages))

	if len(removed) > 0 {
		_, _ = fmt.Fprintln(out, "\nRemoved messages:")
		for _, m := range removed {
			content := m.Content
			if len(content) > 80 {
				content = content[:77] + "..."
			}
			_, _ = fmt.Fprintf(out, "  [%s] %s\n", m.Role, content)
		}
	}

	return nil
}
