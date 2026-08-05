package session

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Format specifies a serialization target.
type Format int

const (
	JSON Format = iota
	Markdown
	HTML
	XML
)

// Serialize encodes a session in the given format.
func Serialize(s *Session, format Format) ([]byte, error) {
	switch format {
	case JSON:
		return ExportJSON(s)
	case Markdown:
		return ExportMarkdown(s)
	case HTML:
		return ExportHTML(s)
	case XML:
		return ExportXML(s)
	default:
		return nil, fmt.Errorf("unsupported format: %d", format)
	}
}

// Deserialize decodes a session from JSON bytes.
func Deserialize(data []byte, format Format) (*Session, error) {
	if format != JSON {
		return nil, fmt.Errorf("deserialize only supports JSON")
	}
	return ImportJSON(data)
}

func ExportJSON(s *Session) ([]byte, error) { return json.MarshalIndent(s, "", "  ") }

func ImportJSON(data []byte) (*Session, error) {
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &s, nil
}

// CompactJSON strips messages and non-essential fields, keeping the envelope.
func CompactJSON(data []byte) ([]byte, error) {
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	s.Messages = nil
	s.Summary = ""
	return json.MarshalIndent(s, "", "  ")
}

// ExportMarkdown renders the full conversation as Markdown.
func ExportMarkdown(s *Session) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "session_id: %s\n", s.ID)
	if s.ParentID != "" {
		fmt.Fprintf(&sb, "parent_id: %s\n", s.ParentID)
	}
	fmt.Fprintf(&sb, "title: %q\nmodel: %s\ncreated_at: %s\nupdated_at: %s\nstatus: %s\nmessages: %d\ntokens: %d\n",
		s.Title, s.Model, s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339),
		s.Status, s.MessageCount, s.TokenCount)
	if len(s.Tags) > 0 {
		fmt.Fprintf(&sb, "tags: [%s]\n", strings.Join(s.Tags, ", "))
	}
	sb.WriteString("---\n\n")
	fmt.Fprintf(&sb, "# %s\n\n", s.Title)

	if s.Summary != "" {
		fmt.Fprintf(&sb, "## Summary\n\n%s\n\n", s.Summary)
	}

	for _, msg := range s.Messages {
		fmt.Fprintf(&sb, "### %s\n*%s*\n\n", cases.Title(language.English).String(string(msg.Role)), msg.Timestamp.Format("2006-01-02 15:04:05"))
		if msg.Content != "" {
			if msg.Role == RoleAssistant {
				fmt.Fprintf(&sb, "```\n%s\n```\n\n", msg.Content)
			} else {
				sb.WriteString(msg.Content + "\n\n")
			}
		}
		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(&sb, "**Tool: %s**\n", tc.Name)
			if tc.Input != "" {
				fmt.Fprintf(&sb, "Input: `%s`\n", tc.Input)
			}
			if tc.Output != "" {
				fmt.Fprintf(&sb, "Output: `%s`\n", tc.Output)
			}
			if tc.Error != "" {
				fmt.Fprintf(&sb, "Error: `%s`\n", tc.Error)
			}
			sb.WriteString("\n")
		}
	}
	return []byte(sb.String()), nil
}

// ExportHTML produces a self-contained HTML page.
func ExportHTML(s *Session) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	fmt.Fprintf(&sb, "<title>%s</title>\n", html.EscapeString(s.Title))
	sb.WriteString("<style>body{font-family:system-ui,sans-serif;max-width:800px;margin:0 auto;padding:2rem;line-height:1.6}\n")
	sb.WriteString("h1{border-bottom:2px solid #333;padding-bottom:.5rem}\n")
	sb.WriteString(".msg{margin:1.5rem 0;padding:1rem;border-radius:8px;border-left:4px solid #ccc}\n")
	sb.WriteString(".msg-user{background:#f0f7ff;border-color:#4a9eff}\n")
	sb.WriteString(".msg-assistant{background:#f5fff0;border-color:#4aff4a}\n")
	sb.WriteString(".msg-system{background:#fff8f0;border-color:#ffaa4a}\n")
	sb.WriteString(".role{font-weight:bold;text-transform:capitalize}\n")
	sb.WriteString(".time{color:#888;font-size:.85em}\n")
	sb.WriteString("pre{background:#f4f4f4;padding:1rem;overflow-x:auto;border-radius:4px}\n")
	sb.WriteString("code{background:#f4f4f4;padding:.15em .3em;border-radius:3px;font-size:.9em}\n")
	sb.WriteString("</style></head><body>\n")

	fmt.Fprintf(&sb, "<h1>%s</h1>\n", html.EscapeString(s.Title))
	fmt.Fprintf(&sb, "<p><strong>Model:</strong> %s &mdash; <strong>Messages:</strong> %d &mdash; <strong>Tokens:</strong> %d</p>\n",
		html.EscapeString(s.Model), s.MessageCount, s.TokenCount)
	fmt.Fprintf(&sb, "<p><em>Created: %s &mdash; Updated: %s &mdash; Status: %s</em></p>\n",
		s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339), s.Status)

	if s.Summary != "" {
		fmt.Fprintf(&sb, "<h2>Summary</h2><div>%s</div>\n", html.EscapeString(s.Summary))
	}

	for _, msg := range s.Messages {
		fmt.Fprintf(&sb, "<div class=\"msg msg-%s\">\n", msg.Role)
		fmt.Fprintf(&sb, "<span class=\"role\">%s</span> <span class=\"time\">%s</span>\n",
			html.EscapeString(string(msg.Role)), msg.Timestamp.Format(time.RFC3339))
		if msg.Content != "" {
			fmt.Fprintf(&sb, "<pre><code>%s</code></pre>\n", html.EscapeString(msg.Content))
		}
		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(&sb, "<div class=\"tool-call\"><strong>Tool: %s</strong>", html.EscapeString(tc.Name))
			if tc.Error != "" {
				fmt.Fprintf(&sb, " <em>(error: %s)</em>", html.EscapeString(tc.Error))
			}
			sb.WriteString("</div>\n")
		}
		sb.WriteString("</div>\n")
	}
	sb.WriteString("</body></html>\n")
	return []byte(sb.String()), nil
}

// ExportXML produces an XML document.
func ExportXML(s *Session) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<session>\n")
	fmt.Fprintf(&sb, "  <id>%s</id>\n  <title>%s</title>\n  <model>%s</model>\n", xmlEsc(s.ID), xmlEsc(s.Title), xmlEsc(s.Model))
	fmt.Fprintf(&sb, "  <status>%s</status>\n  <created_at>%s</created_at>\n", s.Status, s.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&sb, "  <updated_at>%s</updated_at>\n  <message_count>%d</message_count>\n", s.UpdatedAt.Format(time.RFC3339), s.MessageCount)
	fmt.Fprintf(&sb, "  <token_count>%d</token_count>\n", s.TokenCount)
	if s.Summary != "" {
		fmt.Fprintf(&sb, "  <summary>%s</summary>\n", xmlEsc(s.Summary))
	}
	sb.WriteString("  <messages>\n")
	for _, msg := range s.Messages {
		sb.WriteString("    <message>\n")
		fmt.Fprintf(&sb, "      <id>%s</id>\n      <role>%s</role>\n", xmlEsc(msg.ID), msg.Role)
		fmt.Fprintf(&sb, "      <timestamp>%s</timestamp>\n", msg.Timestamp.Format(time.RFC3339))
		if msg.Content != "" {
			fmt.Fprintf(&sb, "      <content>%s</content>\n", xmlEsc(msg.Content))
		}
		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(&sb, "      <tool_call>\n        <name>%s</name>\n        <input>%s</input>\n", xmlEsc(tc.Name), xmlEsc(tc.Input))
			if tc.Output != "" {
				fmt.Fprintf(&sb, "        <output>%s</output>\n", xmlEsc(tc.Output))
			}
			sb.WriteString("      </tool_call>\n")
		}
		sb.WriteString("    </message>\n")
	}
	sb.WriteString("  </messages>\n</session>\n")
	return []byte(sb.String()), nil
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
