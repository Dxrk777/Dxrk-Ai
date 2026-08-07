// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/utils/session"
)

// RegisterSessionCommand registers the `dxrk session` command with subcommands`dxrk session` command with subcommands
// for listing, creating, switching, and deleting sessions.
func RegisterSessionCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage conversation sessions",
		Long:  "List, create, switch, and delete conversation sessions.",
	}

	cmd.AddCommand(
		sessionListCmd(),
		sessionCreateCmd(),
		sessionSwitchCmd(),
		sessionDeleteCmd(),
		sessionInfoCmd(),
	)

	reg.AddCommand(cmd)
}

func sessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".dxrk", "sessions")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create sessions directory: %w", err)
	}
	return dir, nil
}

func listSessionFiles() ([]session.Session, error) {
	dir, err := sessionDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	var sessions []session.Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec
		if err != nil {
			continue
		}
		var s session.Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

func loadSession(id string) (*session.Session, error) {
	dir, err := sessionDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, id+".json")) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("session %q not found", id)
	}

	var s session.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &s, nil
}

func saveSession(s *session.Session) error {
	dir, err := sessionDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	path := filepath.Join(dir, s.ID+".json")
	return os.WriteFile(path, data, 0o600)
}

func sessionListCmd() *cobra.Command {
	var (
		limit  int
		status string
		tag    string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all sessions",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			if status != "" {
				filtered := make([]session.Session, 0)
				for _, s := range sessions {
					if s.Status.String() == status {
						filtered = append(filtered, s)
					}
				}
				sessions = filtered
			}

			if tag != "" {
				filtered := make([]session.Session, 0)
				for _, s := range sessions {
					for _, t := range s.Tags {
						if t == tag {
							filtered = append(filtered, s)
							break
						}
					}
				}
				sessions = filtered
			}

			if limit > 0 && len(sessions) > limit {
				sessions = sessions[:limit]
			}

			if len(sessions) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tTITLE\tMODEL\tMSGS\tSTATUS\tUPDATED")
			for _, s := range sessions {
				title := s.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
					s.ID[:8],
					title,
					s.Model,
					s.MessageCount,
					s.Status,
					s.UpdatedAt.Format("2006-01-02 15:04"),
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of sessions to show")
	cmd.Flags().StringVar(&status, strconst.StrStatus, "", "Filter by status (active, paused, completed, archived)")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")

	return cmd
}

func sessionCreateCmd() *cobra.Command {
	var (
		title string
		model string
		tags  []string
	)

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a new session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && title == "" {
				title = args[0]
			}

			wd, _ := os.Getwd()

			opts := session.SessionOpts{
				Title:      title,
				WorkingDir: wd,
				Model:      model,
			}
			if opts.Model == "" {
				opts.Model = strconst.StrClaudeSonnet420250514
			}

			s := session.NewSession(opts)
			s.Tags = tags

			if err := saveSession(s); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created session %s — %s\n", s.ID[:8], s.Title)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, strconst.StrTitle, "t", "", "Session title")
	cmd.Flags().StringVarP(&model, "model", "m", "", "Model to use")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tags to apply (repeatable)")

	return cmd
}

func sessionSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <session-id>",
		Short: "Switch to a session (sets it as active)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			var found *session.Session
			for i := range sessions {
				if sessions[i].ID == id || strings.HasPrefix(sessions[i].ID, id) {
					s := sessions[i]
					found = &s
					break
				}
			}
			if found == nil {
				return fmt.Errorf("session %q not found", id)
			}

			found.Status = session.Active
			if err := saveSession(found); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Switched to session %s — %s\n", found.ID[:8], found.Title)
			return nil
		},
	}
}

func sessionDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <session-id>",
		Short:   "Delete a session",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			dir, err := sessionDir()
			if err != nil {
				return err
			}

			sessions, err := listSessionFiles()
			if err != nil {
				return err
			}

			for _, s := range sessions {
				if s.ID == id || strings.HasPrefix(s.ID, id) {
					path := filepath.Join(dir, s.ID+".json")
					if err := os.Remove(path); err != nil {
						return fmt.Errorf("delete session: %w", err)
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted session %s — %s\n", s.ID[:8], s.Title)
					return nil
				}
			}
			return fmt.Errorf("session %q not found", id)
		},
	}
}

func sessionInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <session-id>",
		Short: "Show detailed info about a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSession(args[0])
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "ID:          %s\n", s.ID)
			_, _ = fmt.Fprintf(out, "Title:       %s\n", s.Title)
			_, _ = fmt.Fprintf(out, "Model:       %s\n", s.Model)
			_, _ = fmt.Fprintf(out, "Status:      %s\n", s.Status)
			_, _ = fmt.Fprintf(out, "Working Dir: %s\n", s.WorkingDir)
			_, _ = fmt.Fprintf(out, "Created:     %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
			_, _ = fmt.Fprintf(out, "Updated:     %s\n", s.UpdatedAt.Format("2006-01-02 15:04:05"))
			_, _ = fmt.Fprintf(out, "Messages:    %d\n", s.MessageCount)
			_, _ = fmt.Fprintf(out, "Tokens:      %d\n", s.TokenCount)
			_, _ = fmt.Fprintf(out, "Duration:    %s\n", s.Duration().Round(1e6))
			if len(s.Tags) > 0 {
				_, _ = fmt.Fprintf(out, "Tags:        %s\n", strings.Join(s.Tags, ", "))
			}
			if s.Summary != "" {
				_, _ = fmt.Fprintf(out, "Summary:     %s\n", s.Summary)
			}
			return nil
		},
	}
}
