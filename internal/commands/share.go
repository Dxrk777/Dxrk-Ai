// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/utils/session"
)

// RegisterShareCommand registers the `dxrk share` command for sharing`dxrk share` command for sharing
// sessions as files or generating shareable content.
func RegisterShareCommand(reg *Registry) {
	var (
		format string
		output string
		stdout bool
	)

	cmd := &cobra.Command{
		Use:   "share <session-id>",
		Short: "Share session as a file or to stdout",
		Long:  "Export a session for sharing — produces a self-contained markdown or JSON file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSession(args[0])
			if err != nil {
				return err
			}

			var data []byte
			switch format {
			case "json":
				data, err = shareJSON(s)
			default:
				format = strconst.StrMarkdown
				data = shareMarkdown(s)
			}
			if err != nil {
				return err
			}

			if stdout {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}

			if output == "" {
				output = fmt.Sprintf("shared-%s.%s", s.ID[:8], shareExt(format))
			}

			if err := os.WriteFile(output, data, 0o600); err != nil { //nolint:gosec
				return fmt.Errorf("write share file: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Shared session %s to %s (%s)\n", s.ID[:8], output, format)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, strconst.StrFormat, "f", strconst.StrMarkdown, "Share format: markdown, json")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().BoolVar(&stdout, strconst.StrStdout, false, "Print to stdout instead of writing a file")

	reg.AddCommand(cmd)
}

func shareExt(format string) string {
	if format == "json" {
		return "json"
	}
	return "md"
}

func shareJSON(s *session.Session) ([]byte, error) {
	share := map[string]any{
		strconst.StrVersion:   s.Version,
		"id":                  s.ID,
		strconst.StrTitle:     s.Title,
		"model":               s.Model,
		strconst.StrCreatedAt: s.CreatedAt,
		"updated_at":          s.UpdatedAt,
		"message_count":       s.MessageCount,
		"token_count":         s.TokenCount,
		"messages":            s.Messages,
	}
	return json.MarshalIndent(share, "", "  ")
}

func shareMarkdown(s *session.Session) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Title)
	fmt.Fprintf(&b, "> Shared from Dxrk AI — Model: %s | Messages: %d | Tokens: %d\n\n",
		s.Model, s.MessageCount, s.TokenCount)

	for _, msg := range s.Messages {
		role := string(msg.Role)
		switch msg.Role {
		case "user":
			role = "**User**"
		case strconst.StrAssistant:
			role = "**Assistant**"
		case strconst.StrSystem:
			role = "**System**"
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", role, msg.Content)
	}

	return []byte(b.String())
}
