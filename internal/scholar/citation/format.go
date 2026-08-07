// SPDX-License-Identifier: MIT

package citation

import (
	"fmt"
	"strings"
)

// Paper mirrors the citation-relevant fields of a scholarly publication. It is
// defined locally to avoid an import cycle with the scholar package.
type Paper struct {
	Title    string
	Authors  []string
	DOI      string
	Abstract string
	URL      string
	Year     int
}

// FormatBibTeX renders a paper as a BibTeX @article entry keyed by first author
// and year.
func FormatBibTeX(p Paper) string {
	key := "paper"
	if len(p.Authors) > 0 {
		last := lastName(p.Authors[0])
		key = normalizeKey(last)
		if p.Year > 0 {
			key = key + fmt.Sprintf("%d", p.Year)
		}
	}

	var b strings.Builder
	b.WriteString("@article{" + key + ",\n")
	if p.Title != "" {
		b.WriteString("\ttitle = {" + escapeBibTeX(p.Title) + "},\n")
	}
	if len(p.Authors) > 0 {
		b.WriteString("\tauthor = {" + formatBibTeXAuthors(p.Authors) + "},\n")
	}
	if p.Year > 0 {
		b.WriteString(fmt.Sprintf("\tyear = {%d},\n", p.Year))
	}
	if p.DOI != "" {
		b.WriteString("\tdoi = {" + p.DOI + "},\n")
	}
	if p.Abstract != "" {
		b.WriteString("\tabstract = {" + escapeBibTeX(p.Abstract) + "},\n")
	}
	if p.URL != "" {
		b.WriteString("\turl = {" + p.URL + "},\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// FormatAPA renders an APA-7 style reference entry.
func FormatAPA(p Paper) string {
	var b strings.Builder
	for i, a := range p.Authors {
		if i > 0 {
			if i == len(p.Authors)-1 && len(p.Authors) > 2 {
				b.WriteString(", & ")
			} else {
				b.WriteString(", ")
			}
		}
		b.WriteString(fmtAPAInvert(a))
	}
	if len(p.Authors) == 0 {
		b.WriteString("Anonymous")
	}
	b.WriteString(". ")
	if p.Year > 0 {
		b.WriteString(fmt.Sprintf("(%d). ", p.Year))
	} else {
		b.WriteString("(n.d.). ")
	}
	if p.Title != "" {
		b.WriteString(italicize(p.Title))
	}
	if p.DOI != "" {
		b.WriteString(" https://doi.org/" + p.DOI)
	} else if p.URL != "" {
		b.WriteString(" " + p.URL)
	}
	return b.String()
}

// FormatMLA renders an MLA-style Works Cited entry.
func FormatMLA(p Paper) string {
	var b strings.Builder
	for i, a := range p.Authors {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmtMLA(a, i == 0))
	}
	if len(p.Authors) > 0 {
		b.WriteString(". ")
	}
	if p.Title != "" {
		b.WriteString(p.Title)
		if p.Year > 0 {
			b.WriteString(fmt.Sprintf(". %d. ", p.Year))
		} else {
			b.WriteString(". n.d. ")
		}
	}
	if p.URL != "" {
		b.WriteString(p.URL)
	}
	return b.String()
}

func lastName(full string) string {
	parts := strings.Fields(full)
	if len(parts) == 0 {
		return full
	}
	return parts[len(parts)-1]
}

func normalizeKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r == ' ' || r == '-':
			// skip separators
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "paper"
	}
	return b.String()
}

func escapeBibTeX(s string) string {
	replacer := strings.NewReplacer(
		"&", `\&`,
		"%", `\%`,
		"$", `\$`,
		"#", `\#`,
		"_", `\_`,
	)
	return replacer.Replace(s)
}

func formatBibTeXAuthors(authors []string) string {
	out := make([]string, 0, len(authors))
	for _, a := range authors {
		if parts := strings.Fields(strings.TrimSpace(a)); len(parts) > 1 {
			out = append(out, parts[len(parts)-1]+", "+strings.Join(parts[:len(parts)-1], " "))
		} else {
			out = append(out, strings.TrimSpace(a))
		}
	}
	return strings.Join(out, " and ")
}

// fmtAPAInvert renders "LastName, F. M.".
func fmtAPAInvert(full string) string {
	parts := strings.Fields(strings.TrimSpace(full))
	if len(parts) == 0 {
		return full
	}
	last := parts[len(parts)-1]
	prefixes := []string{"van", "von", "de", "del", "la"}
	if len(parts) > 1 && contains(prefixes, parts[0]) {
		last = parts[0] + " " + last
		parts = parts[1:]
	}
	if len(parts) <= 1 {
		return last
	}
	initials := make([]string, 0, len(parts)-1)
	for _, p := range parts[:len(parts)-1] {
		initials = append(initials, string(p[0])+".")
	}
	return last + ", " + strings.Join(initials, " ")
}

// fmtMLA renders "Last, First" (first author) or "First Last".
func fmtMLA(full string, first bool) string {
	if !first {
		return strings.TrimSpace(full)
	}
	parts := strings.Fields(strings.TrimSpace(full))
	if len(parts) <= 1 {
		return strings.TrimSpace(full)
	}
	return parts[len(parts)-1] + ", " + strings.Join(parts[:len(parts)-1], " ")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func italicize(s string) string {
	return s
}
