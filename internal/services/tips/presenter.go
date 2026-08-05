package tips

import (
	"fmt"
	"strings"
)

// Presenter formats tips for display.
type Presenter struct {
	style TipStyle
}

// TipStyle configures how tips are rendered.
type TipStyle struct {
	UseEmoji    bool
	UseColors   bool
	Width       int
	CompactMode bool
}

func NewPresenter(style TipStyle) *Presenter {
	if style.Width <= 0 {
		style.Width = 60
	}
	return &Presenter{style: style}
}

// FormatTip formats a single tip for display.
func (p *Presenter) FormatTip(tip Tip) string {
	var b strings.Builder

	if p.style.UseEmoji {
		b.WriteString("💡 ")
	}

	b.WriteString(tip.Title)
	b.WriteString("\n")

	if !p.style.CompactMode {
		b.WriteString(strings.Repeat("-", min(len(tip.Title)+2, p.style.Width)))
		b.WriteString("\n")
	}

	content := tip.Content
	if p.style.Width > 0 && !p.style.CompactMode {
		content = wrapText(content, p.style.Width)
	}
	b.WriteString(content)

	return b.String()
}

// FormatTipBox formats a tip in a box.
func (p *Presenter) FormatTipBox(tip Tip) string {
	width := p.style.Width
	if width < 40 {
		width = 60
	}

	var b strings.Builder

	// Top border
	b.WriteString("┌")
	b.WriteString(strings.Repeat("─", width-2))
	b.WriteString("┐\n")

	// Title line
	title := tip.Title
	if p.style.UseEmoji {
		title = "💡 " + title
	}
	if len(title) > width-4 {
		title = title[:width-7] + "..."
	}
	padding := width - 4 - len(title)
	b.WriteString("│ ")
	b.WriteString(title)
	b.WriteString(strings.Repeat(" ", padding))
	b.WriteString(" │\n")

	// Separator
	b.WriteString("├")
	b.WriteString(strings.Repeat("─", width-2))
	b.WriteString("┤\n")

	// Content wrapped
	words := strings.Fields(tip.Content)
	line := ""
	for _, word := range words {
		if len(line)+len(word)+1 > width-4 {
			if line != "" {
				lp := width - 4 - len(line)
				if lp < 0 {
					lp = 0
				}
				b.WriteString("│ ")
				b.WriteString(line)
				b.WriteString(strings.Repeat(" ", lp))
				b.WriteString(" │\n")
			}
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	if line != "" {
		lp := width - 4 - len(line)
		if lp < 0 {
			lp = 0
		}
		b.WriteString("│ ")
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", lp))
		b.WriteString(" │\n")
	}

	// Category line
	catStr := fmt.Sprintf("[%s]", tip.Category)
	catPad := width - 4 - len(catStr)
	if catPad < 0 {
		catPad = 0
	}
	b.WriteString("│ ")
	b.WriteString(catStr)
	b.WriteString(strings.Repeat(" ", catPad))
	b.WriteString(" │\n")

	// Bottom border
	b.WriteString("└")
	b.WriteString(strings.Repeat("─", width-2))
	b.WriteString("┘")

	return b.String()
}

// FormatSummary formats a tips stats summary.
func (p *Presenter) FormatSummary(stats TipsStats) string {
	var b strings.Builder

	if p.style.UseEmoji {
		b.WriteString("📊 ")
	}
	b.WriteString("Tips Statistics\n")
	b.WriteString(strings.Repeat("─", 35))
	b.WriteString("\n")

	fmt.Fprintf(&b, "  Total:      %d\n", stats.Total)
	fmt.Fprintf(&b, "  Enabled:    %d\n", stats.Enabled)
	fmt.Fprintf(&b, "  Disabled:   %d\n", stats.Disabled)
	fmt.Fprintf(&b, "  Session:    %d shown\n", stats.ShownThisSession)
	fmt.Fprintf(&b, "  All-time:   %d shown\n", stats.TotalShownAllTime)

	if len(stats.ByCategory) > 0 {
		b.WriteString("\nBy Category:\n")
		cats := make([]string, 0, len(stats.ByCategory))
		for cat := range stats.ByCategory {
			cats = append(cats, cat)
		}
		for _, cat := range cats {
			fmt.Fprintf(&b, "  %-15s %d\n", cat, stats.ByCategory[cat])
		}
	}

	return b.String()
}

// FormatCategoryGroup formats tips grouped by category.
func (p *Presenter) FormatCategoryGroup(tips []Tip) string {
	if len(tips) == 0 {
		return "No tips available."
	}

	groups := make(map[string][]Tip)
	for _, tip := range tips {
		groups[tip.Category] = append(groups[tip.Category], tip)
	}

	var b strings.Builder
	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	for _, cat := range categories {
		fmt.Fprintf(&b, "── %s ──\n", cat)
		for _, tip := range groups[cat] {
			b.WriteString("  • ")
			b.WriteString(tip.Title)
			b.WriteString("\n")
			if !p.style.CompactMode {
				b.WriteString("    ")
				b.WriteString(truncateStr(tip.Content, 80))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+len(word)+1 > width {
			lines = append(lines, line)
			line = word
		} else {
			line += " " + word
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
