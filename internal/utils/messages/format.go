package messages

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// FormatStyle controls the output style of FormatMessage.
type FormatStyle int

const (
	Plain FormatStyle = iota
	Markdown
	Rich
	Compact
	Verbose
)

// FormatMessage renders a Message according to the given style.
func FormatMessage(msg Message, format FormatStyle) string {
	switch format {
	case Plain:
		return formatPlain(msg)
	case Markdown:
		return formatMarkdown(msg)
	case Rich:
		return formatRich(msg)
	case Compact:
		return formatCompact(msg)
	case Verbose:
		return formatVerbose(msg)
	default:
		return formatPlain(msg)
	}
}

func formatPlain(msg Message) string {
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(msg.Role.String())
	sb.WriteString("] ")
	sb.WriteString(msg.TextContent())
	return sb.String()
}

func formatMarkdown(msg Message) string {
	var sb strings.Builder
	sb.WriteString("**")
	sb.WriteString(strings.ToUpper(msg.Role.String()))
	sb.WriteString("**\n\n")

	for _, c := range msg.Contents {
		switch c.Type {
		case ContentText:
			sb.WriteString(c.Text)
			sb.WriteString("\n")
		case ContentImage:
			if c.Image != nil {
				sb.WriteString("[Image: ")
				sb.WriteString(c.Image.MediaType)
				sb.WriteString("]\n")
			}
		case ContentToolUse:
			if c.ToolUse != nil {
				sb.WriteString("🔧 ")
				sb.WriteString(c.ToolUse.Name)
				sb.WriteString("(```json\n")
				sb.WriteString(formatToolInput(c.ToolUse.Input))
				sb.WriteString("\n```)  \n")
			}
		case ContentToolResult:
			if c.ToolResult != nil {
				prefix := "✅"
				if c.ToolResult.IsError {
					prefix = "❌"
				}
				sb.WriteString(prefix)
				sb.WriteString(" ")
				sb.WriteString(truncStr(c.ToolResult.Content, 200))
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func formatRich(msg Message) string {
	var sb strings.Builder
	ts := msg.Timestamp.Format("15:04:05")
	fmt.Fprintf(&sb, "[%s] %s: ", ts, msg.Role)

	parts := make([]string, 0, len(msg.Contents))
	for _, c := range msg.Contents {
		switch c.Type {
		case ContentText:
			parts = append(parts, c.Text)
		case ContentImage:
			parts = append(parts, "[image]")
		case ContentToolUse:
			if c.ToolUse != nil {
				parts = append(parts, fmt.Sprintf("→ %s", c.ToolUse.Name))
			}
		case ContentToolResult:
			if c.ToolResult != nil {
				status := "ok"
				if c.ToolResult.IsError {
					status = strconst.StrError
				}
				parts = append(parts, fmt.Sprintf("← %s", status))
			}
		}
	}
	sb.WriteString(strings.Join(parts, " | "))

	if msg.StopReason != "" {
		fmt.Fprintf(&sb, " [%s]", msg.StopReason)
	}
	return sb.String()
}

func formatCompact(msg Message) string {
	role := string([]byte{msg.Role.String()[0]})
	text := msg.TextContent()
	text = truncStr(text, 80)
	return fmt.Sprintf("%s: %s", role, text)
}

func formatVerbose(msg Message) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Message ID:    %s\n", msg.ID)
	fmt.Fprintf(&sb, "Role:          %s\n", msg.Role)
	fmt.Fprintf(&sb, "Timestamp:     %s\n", msg.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&sb, "Model:         %s\n", msg.Model)
	fmt.Fprintf(&sb, "Tokens:        %d\n", msg.TokenCount)
	fmt.Fprintf(&sb, "Stop Reason:   %s\n", msg.StopReason)
	fmt.Fprintf(&sb, "Contents (%d):\n", len(msg.Contents))

	for i, c := range msg.Contents {
		fmt.Fprintf(&sb, "  [%d] Type: %s\n", i, c.Type)
		switch c.Type {
		case ContentText:
			fmt.Fprintf(&sb, "       Text: %s\n", truncStr(c.Text, 120))
		case ContentImage:
			if c.Image != nil {
				fmt.Fprintf(&sb, "       Image: %s (%s)\n", c.Image.MediaType, c.Image.Source)
			}
		case ContentToolUse:
			if c.ToolUse != nil {
				fmt.Fprintf(&sb, "       Tool: %s (id=%s)\n", c.ToolUse.Name, c.ToolUse.ID)
				fmt.Fprintf(&sb, "       Input: %s\n", truncStr(formatToolInput(c.ToolUse.Input), 200))
			}
		case ContentToolResult:
			if c.ToolResult != nil {
				fmt.Fprintf(&sb, "       Result: tool_use_id=%s error=%v\n", c.ToolResult.ToolUseID, c.ToolResult.IsError)
				fmt.Fprintf(&sb, "       Content: %s\n", truncStr(c.ToolResult.Content, 200))
			}
		}
	}

	if len(msg.Metadata) > 0 {
		sb.WriteString("Metadata:\n")
		for k, v := range msg.Metadata {
			fmt.Fprintf(&sb, "  %s: %v\n", k, v)
		}
	}
	return sb.String()
}

func formatToolInput(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for k, v := range input {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&sb, "%q: %q", k, fmt.Sprintf("%v", v))
	}
	sb.WriteString("}")
	return sb.String()
}

// FormatToolUse formats a tool call for display.
func FormatToolUse(name string, input map[string]any) string {
	return fmt.Sprintf("→ %s(%s)", name, formatToolInput(input))
}

// FormatToolResult formats a tool result for display.
func FormatToolResult(result ToolResultData) string {
	prefix := "✓"
	if result.IsError {
		prefix = "✗"
	}
	content := truncStr(result.Content, 100)
	dur := ""
	if result.Duration > 0 {
		dur = fmt.Sprintf(" [%s]", result.Duration.Round(time.Millisecond))
	}
	return fmt.Sprintf("%s %s%s", prefix, content, dur)
}

// FormatError formats an error message with context.
func FormatError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("Error: %s", err.Error())
}

// FormatProgress returns a progress indicator string.
func FormatProgress(tool string, elapsed time.Duration) string {
	sec := elapsed.Seconds()
	dots := int(sec) % 4
	pending := strings.Repeat(".", dots+1)
	return fmt.Sprintf("  %s%s %s", tool, pending, elapsed.Round(time.Millisecond))
}

// FormatDiff produces a simple before/after comparison.
func FormatDiff(before, after string) string {
	var sb strings.Builder
	sb.WriteString("--- before\n")
	sb.WriteString(before)
	sb.WriteString("\n+++ after\n")
	sb.WriteString(after)
	return sb.String()
}

// TruncateMiddle truncates a string to maxLen, placing ellipsis in the middle.
func TruncateMiddle(s string, maxLen int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen <= maxLen {
		return s
	}
	if maxLen < 5 {
		return s[:maxLen]
	}
	half := (maxLen - 3) / 2
	start := truncateToN(s, half)
	end := truncateFromEnd(s, half)
	return start + "..." + end
}

func truncateToN(s string, n int) string {
	count := 0
	for i := range s {
		count++
		if count > n {
			return s[:i]
		}
	}
	return s
}

func truncateFromEnd(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[len(runes)-n:])
}

// WrapCode wraps text in a markdown code block.
func WrapCode(code string, lang string) string {
	return fmt.Sprintf("```%s\n%s\n```", lang, code)
}

// StripANSI removes ANSI escape codes from a string.
func StripANSI(s string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiRegex.ReplaceAllString(s, "")
}

// WordCount returns the number of words in s.
func WordCount(s string) int {
	return len(strings.Fields(s))
}

// CharCount returns the rune count of s.
func CharCount(s string) int {
	return utf8.RuneCountInString(s)
}

// truncStr truncates s to maxLen runes with "..." suffix.
func truncStr(s string, maxLen int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	runes := []rune(s)
	return string(runes[:maxLen-3]) + "..."
}
