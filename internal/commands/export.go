// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/utils/session"
)

// RegisterExportCommand registers the `dxrk export` command for exporting`dxrk export` command for exporting
// sessions in markdown, JSON, or HTML format.
func RegisterExportCommand(reg *Registry) {
	var (
		format string
		output string
	)

	cmd := &cobra.Command{
		Use:   "export <session-id>",
		Short: "Export session to a file",
		Long:  "Export a conversation session as markdown, JSON, or HTML.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSession(args[0])
			if err != nil {
				return err
			}

			// Determine output path.
			if output == "" {
				safeTitle := strings.ReplaceAll(s.Title, " ", "_")
				safeTitle = strings.Map(func(r rune) rune {
					if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
						return '_'
					}
					return r
				}, safeTitle)
				if len(safeTitle) > 50 {
					safeTitle = safeTitle[:50]
				}
				output = fmt.Sprintf("session-%s.%s", safeTitle, exportExt(format))
			}

			var data []byte
			switch format {
			case "json":
				data, err = exportJSON(s)
			case "html":
				data = exportHTML(s)
			default:
				format = strconst.StrMarkdown
				data = exportMarkdown(s)
			}
			if err != nil {
				return err
			}

			if err := os.WriteFile(output, data, 0o600); err != nil { //nolint:gosec
				return fmt.Errorf("write export file: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exported session %s to %s (%s)\n", s.ID[:8], output, format)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, strconst.StrFormat, "f", strconst.StrMarkdown, "Export format: markdown, json, html")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (default: auto-generated)")

	reg.AddCommand(cmd)
}

func exportExt(format string) string {
	switch format {
	case "json":
		return "json"
	case "html":
		return "html"
	default:
		return "md"
	}
}

func exportJSON(s *session.Session) ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func exportMarkdown(s *session.Session) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Title)
	fmt.Fprintf(&b, "- **ID**: %s\n", s.ID)
	fmt.Fprintf(&b, "- **Model**: %s\n", s.Model)
	fmt.Fprintf(&b, "- **Status**: %s\n", s.Status)
	fmt.Fprintf(&b, "- **Created**: %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "- **Updated**: %s\n", s.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "- **Messages**: %d\n", s.MessageCount)
	fmt.Fprintf(&b, "- **Tokens**: %d\n", s.TokenCount)
	if len(s.Tags) > 0 {
		fmt.Fprintf(&b, "- **Tags**: %s\n", strings.Join(s.Tags, ", "))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "---")
	fmt.Fprintln(&b)

	for _, msg := range s.Messages {
		role := string(msg.Role)
		switch msg.Role {
		case "user":
			role = "👤 User"
		case strconst.StrAssistant:
			role = "🤖 Assistant"
		case strconst.StrSystem:
			role = "⚙️ System"
		}
		fmt.Fprintf(&b, "### %s\n\n", role)
		fmt.Fprintf(&b, "%s\n\n", msg.Content)
		fmt.Fprintln(&b, "---")
		fmt.Fprintln(&b)
	}

	return []byte(b.String())
}

func exportHTML(s *session.Session) []byte {
	var b strings.Builder

	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"UTF-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", escapeHTML(s.Title))
	b.WriteString(`<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 0 auto; padding: 2rem; line-height: 1.6; color: #333; }
h1 { border-bottom: 2px solid #eee; padding-bottom: 0.5rem; }
.meta { color: #666; font-size: 0.9em; margin-bottom: 2rem; }
.meta dt { font-weight: bold; display: inline; }
.meta dd { display: inline; margin: 0; }
.message { margin: 1.5rem 0; padding: 1rem; border-radius: 8px; }
.user { background: #f0f7ff; border-left: 4px solid #3b82f6; }
.assistant { background: #f0fdf4; border-left: 4px solid #22c55e; }
.system { background: #fefce8; border-left: 4px solid #eab308; }
.role { font-weight: bold; margin-bottom: 0.5rem; }
.content { white-space: pre-wrap; }
</style>
</head>
<body>
`)
	fmt.Fprintf(&b, "<h1>%s</h1>\n", escapeHTML(s.Title))
	b.WriteString("<dl class=\"meta\">\n")
	fmt.Fprintf(&b, "  <dt>ID:</dt><dd>%s</dd>\n", s.ID)
	fmt.Fprintf(&b, "  <dt>Model:</dt><dd>%s</dd>\n", s.Model)
	fmt.Fprintf(&b, "  <dt>Created:</dt><dd>%s</dd>\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "  <dt>Messages:</dt><dd>%d</dd>\n", s.MessageCount)
	b.WriteString("</dl>\n<hr>\n")

	for _, msg := range s.Messages {
		cls := string(msg.Role)
		role := string(msg.Role)
		switch msg.Role {
		case "user":
			role = "👤 User"
		case strconst.StrAssistant:
			role = "🤖 Assistant"
		case strconst.StrSystem:
			role = "⚙️ System"
		}
		fmt.Fprintf(&b, "<div class=\"message %s\">\n", cls)
		fmt.Fprintf(&b, "  <div class=\"role\">%s</div>\n", role)
		fmt.Fprintf(&b, "  <div class=\"content\">%s</div>\n", escapeHTML(msg.Content))
		b.WriteString("</div>\n")
	}

	b.WriteString("</body>\n</html>\n")
	return []byte(b.String())
}

func escapeHTML(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExportFilePath returns the default export path for a session.
func ExportFilePath(s *session.Session, format string) string {
	safeTitle := strings.ReplaceAll(s.Title, " ", "_")
	safeTitle = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, safeTitle)
	if len(safeTitle) > 50 {
		safeTitle = safeTitle[:50]
	}
	return filepath.Join(".", fmt.Sprintf("session-%s.%s", safeTitle, exportExt(format)))
}
