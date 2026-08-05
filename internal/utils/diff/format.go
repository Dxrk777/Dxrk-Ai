package diff

import (
	"encoding/json"
	"fmt"
	"strings"
)

var useColors bool

// SetColors enables or disables ANSI color codes in output.
func SetColors(enabled bool) { useColors = enabled }

const (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorDim   = "\033[2m"
)

func color(code, s string) string {
	if !useColors {
		return s
	}
	return code + s + colorReset
}

// FormatUnified produces a unified diff string (git diff style).
func FormatUnified(diff *DiffResult, contextLines int) string {
	if diff == nil || len(diff.Hunks) == 0 {
		return ""
	}
	var b strings.Builder
	for i := range diff.Hunks {
		h := &diff.Hunks[i]
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, line := range h.Lines {
			switch line.Type {
			case DiffEqual:
				b.WriteString(color(colorDim, " "+line.Content))
				b.WriteByte('\n')
			case DiffDelete:
				b.WriteString(color(colorRed, "-"+line.Content))
				b.WriteByte('\n')
			case DiffInsert:
				b.WriteString(color(colorGreen, "+"+line.Content))
				b.WriteByte('\n')
			case DiffModify:
				parts := strings.SplitN(line.Content, "\x00", 2)
				b.WriteString(color(colorRed, "-"+parts[0]))
				b.WriteByte('\n')
				if len(parts) > 1 {
					b.WriteString(color(colorGreen, "+"+parts[1]))
					b.WriteByte('\n')
				}
			}
		}
	}
	return b.String()
}

// FormatContext produces a context diff string.
func FormatContext(diff *DiffResult) string {
	if diff == nil || len(diff.Hunks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, h := range diff.Hunks {
		fmt.Fprintf(&b, "***************\n*** %d,%d ****\n", h.OldStart, h.OldStart+h.OldCount-1)
		for _, line := range h.Lines {
			switch line.Type {
			case DiffEqual:
				b.WriteString("  " + line.Content + "\n")
			case DiffDelete:
				b.WriteString("- " + line.Content + "\n")
			case DiffModify:
				parts := strings.SplitN(line.Content, "\x00", 2)
				b.WriteString("- " + parts[0] + "\n")
			}
		}
		fmt.Fprintf(&b, "--- %d,%d ----\n", h.NewStart, h.NewStart+h.NewCount-1)
		for _, line := range h.Lines {
			switch line.Type {
			case DiffEqual:
				b.WriteString("  " + line.Content + "\n")
			case DiffInsert:
				b.WriteString("+ " + line.Content + "\n")
			case DiffModify:
				parts := strings.SplitN(line.Content, "\x00", 2)
				if len(parts) > 1 {
					b.WriteString("+ " + parts[1] + "\n")
				}
			}
		}
	}
	return b.String()
}

// FormatSideBySide produces a two-column diff view.
func FormatSideBySide(diff *DiffResult, width int) string {
	if diff == nil || len(diff.Hunks) == 0 || width < 40 {
		width = 80
	}
	half := (width - 3) / 2
	var b strings.Builder
	for _, h := range diff.Hunks {
		for _, line := range h.Lines {
			left, right := "", ""
			switch line.Type {
			case DiffEqual:
				left, right = line.Content, line.Content
			case DiffDelete:
				left = line.Content
			case DiffInsert:
				right = line.Content
			case DiffModify:
				parts := strings.SplitN(line.Content, "\x00", 2)
				left = parts[0]
				if len(parts) > 1 {
					right = parts[1]
				}
			}
			fmt.Fprintf(&b, "%s | %s\n", padRight(left, half), padRight(right, half))
		}
	}
	return b.String()
}

// FormatCompact produces a one-line summary per hunk.
func FormatCompact(diff *DiffResult) string {
	if diff == nil || len(diff.Hunks) == 0 {
		return ""
	}
	var b strings.Builder
	for i, h := range diff.Hunks {
		added, removed, changed := countHunkStats(h)
		fmt.Fprintf(&b, "hunk %d: -%d +%d ~%d @ line %d\n", i+1, removed, added, changed, h.OldStart)
	}
	return b.String()
}

// FormatMarkdown produces a Markdown-formatted diff.
func FormatMarkdown(diff *DiffResult) string {
	if diff == nil || len(diff.Hunks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("```diff\n")
	for _, h := range diff.Hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, line := range h.Lines {
			switch line.Type {
			case DiffEqual:
				b.WriteString(" " + line.Content + "\n")
			case DiffDelete:
				b.WriteString("-" + line.Content + "\n")
			case DiffInsert:
				b.WriteString("+" + line.Content + "\n")
			case DiffModify:
				parts := strings.SplitN(line.Content, "\x00", 2)
				b.WriteString("-" + parts[0] + "\n")
				if len(parts) > 1 {
					b.WriteString("+" + parts[1] + "\n")
				}
			}
		}
	}
	b.WriteString("```\n")
	return b.String()
}

// FormatHTML produces an HTML-formatted diff with color coding.
func FormatHTML(diff *DiffResult) string {
	if diff == nil || len(diff.Hunks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<div class=\"diff\">\n")
	for _, h := range diff.Hunks {
		fmt.Fprintf(&b, "<div class=\"hunk\" data-old=\"%d\" data-new=\"%d\">\n", h.OldStart, h.NewStart)
		for _, line := range h.Lines {
			cls, prefix := "equal", " "
			switch line.Type {
			case DiffDelete:
				cls, prefix = "delete", "-"
			case DiffInsert:
				cls, prefix = "insert", "+"
			case DiffModify:
				cls, prefix = "modify", "~"
			}
			fmt.Fprintf(&b, "<div class=\"line %s\">%s%s</div>\n", cls, prefix, htmlEscape(line.Content))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
	return b.String()
}

// FormatJSON produces a structured JSON representation of the diff.
func FormatJSON(diff *DiffResult) ([]byte, error) {
	type jsonLine struct {
		Type    string `json:"type"`
		OldLine int    `json:"old_line,omitempty"`
		NewLine int    `json:"new_line,omitempty"`
		Content string `json:"content"`
	}
	type jsonHunk struct {
		OldStart int        `json:"old_start"`
		OldCount int        `json:"old_count"`
		NewStart int        `json:"new_start"`
		NewCount int        `json:"new_count"`
		Lines    []jsonLine `json:"lines"`
	}
	type jsonResult struct {
		Hunks []jsonHunk `json:"hunks"`
		Stats DiffStats  `json:"stats"`
	}
	result := jsonResult{Hunks: make([]jsonHunk, 0, len(diff.Hunks)), Stats: diff.Stats}
	for _, h := range diff.Hunks {
		jh := jsonHunk{OldStart: h.OldStart, OldCount: h.OldCount, NewStart: h.NewStart, NewCount: h.NewCount,
			Lines: make([]jsonLine, 0, len(h.Lines))}
		for _, l := range h.Lines {
			jh.Lines = append(jh.Lines, jsonLine{Type: l.Type.String(), OldLine: l.LineNumOld, NewLine: l.LineNumNew, Content: l.Content})
		}
		result.Hunks = append(result.Hunks, jh)
	}
	return json.MarshalIndent(result, "", "  ")
}

// FormatHeaders returns git-style --- and +++ headers.
func FormatHeaders(oldName, newName string) string {
	return color(colorRed, "--- "+oldName) + "\n" + color(colorGreen, "+++ "+newName) + "\n"
}

// FormatWithLineNumbers returns a unified diff with line numbers.
func FormatWithLineNumbers(diff *DiffResult, contextLines int) string {
	if diff == nil || len(diff.Hunks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, h := range diff.Hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		oldNum, newNum := h.OldStart, h.NewStart
		for _, line := range h.Lines {
			switch line.Type {
			case DiffEqual:
				fmt.Fprintf(&b, "%s %s %s\n",
					color(colorDim, fmt.Sprintf("%4d", oldNum)),
					color(colorDim, fmt.Sprintf("%4d", newNum)), line.Content)
				oldNum++
				newNum++
			case DiffDelete:
				fmt.Fprintf(&b, "%s      %s\n", color(colorRed, fmt.Sprintf("%4d", oldNum)), color(colorRed, "-"+line.Content))
				oldNum++
			case DiffInsert:
				fmt.Fprintf(&b, "      %s %s\n", color(colorGreen, fmt.Sprintf("%4d", newNum)), color(colorGreen, "+"+line.Content))
				newNum++
			case DiffModify:
				parts := strings.SplitN(line.Content, "\x00", 2)
				fmt.Fprintf(&b, "%s      %s\n", color(colorRed, fmt.Sprintf("%4d", oldNum)), color(colorRed, "-"+parts[0]))
				if len(parts) > 1 {
					fmt.Fprintf(&b, "      %s %s\n", color(colorGreen, fmt.Sprintf("%4d", newNum)), color(colorGreen, "+"+parts[1]))
				}
				oldNum++
				newNum++
			}
		}
	}
	return b.String()
}

func countHunkStats(h DiffHunk) (added, removed, changed int) {
	for _, line := range h.Lines {
		switch line.Type {
		case DiffInsert:
			added++
		case DiffDelete:
			removed++
		case DiffModify:
			changed++
		}
	}
	return
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
