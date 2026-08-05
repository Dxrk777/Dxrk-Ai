package diff

// DiffType represents the type of a diff line.
type DiffType int

const (
	DiffEqual DiffType = iota
	DiffInsert
	DiffDelete
	DiffModify
)

func (d DiffType) String() string {
	return [...]string{"equal", "insert", "delete", "modify"}[d]
}

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type       DiffType
	LineNumOld int
	LineNumNew int
	Content    string
}

// DiffHunk represents a group of related changes.
type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
	Context  string
}

// DiffStats holds summary statistics for a diff.
type DiffStats struct {
	LinesAdded   int
	LinesRemoved int
	LinesChanged int
	TotalLines   int
}

// DiffResult holds the complete diff between two texts.
type DiffResult struct {
	Hunks []DiffHunk
	Stats DiffStats
}

// DefaultContextLines is the default number of context lines around changes.
const DefaultContextLines = 3

// ComputeDiff performs a line-level diff between oldText and newText.
func ComputeDiff(oldText, newText string) *DiffResult {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	ops := lcsDiff(oldLines, newLines)
	return buildResult(ops, oldLines, newLines, DefaultContextLines)
}

// ComputeWordDiff performs a word-level diff between oldText and newText.
func ComputeWordDiff(oldText, newText string) *DiffResult {
	oldWords := splitWords(oldText)
	newWords := splitWords(newText)
	ops := lcsDiff(oldWords, newWords)
	return buildResult(ops, oldWords, newWords, DefaultContextLines)
}

// ComputeCharDiff performs a character-level diff between oldText and newText.
func ComputeCharDiff(oldText, newText string) *DiffResult {
	oldChars := splitChars(oldText)
	newChars := splitChars(newText)
	ops := lcsDiff(oldChars, newChars)
	return buildResult(ops, oldChars, newChars, DefaultContextLines)
}

type op struct {
	typ    DiffType
	oldIdx int
	newIdx int
}

func lcsDiff(oldArr, newArr []string) []op {
	n, m := len(oldArr), len(newArr)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			switch {
			case oldArr[i-1] == newArr[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	ops := make([]op, 0, n+m)
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldArr[i-1] == newArr[j-1]:
			ops = append(ops, op{DiffEqual, i - 1, j - 1})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			ops = append(ops, op{DiffInsert, -1, j - 1})
			j--
		default:
			ops = append(ops, op{DiffDelete, i - 1, -1})
			i--
		}
	}
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

func buildResult(ops []op, oldLines, newLines []string, context int) *DiffResult {
	lines := make([]DiffLine, 0, len(ops))
	stats := DiffStats{}
	for _, o := range ops {
		switch o.typ {
		case DiffEqual:
			lines = append(lines, DiffLine{DiffEqual, o.oldIdx + 1, o.newIdx + 1, oldLines[o.oldIdx]})
		case DiffDelete:
			lines = append(lines, DiffLine{DiffDelete, o.oldIdx + 1, 0, oldLines[o.oldIdx]})
			stats.LinesRemoved++
		case DiffInsert:
			lines = append(lines, DiffLine{DiffInsert, 0, o.newIdx + 1, newLines[o.newIdx]})
			stats.LinesAdded++
		}
	}

	// Mark consecutive delete+insert pairs as Modify.
	merged := make([]DiffLine, 0, len(lines))
	i := 0
	for i < len(lines) {
		if lines[i].Type == DiffDelete {
			j := i + 1
			for j < len(lines) && lines[j].Type == DiffInsert {
				j++
			}
			delCount := 0
			for k := i; k < j; k++ {
				if lines[k].Type == DiffDelete {
					delCount++
				}
			}
			insertCount := j - i - delCount
			minCount := delCount
			if insertCount < minCount {
				minCount = insertCount
			}
			for k := 0; k < minCount; k++ {
				merged = append(merged, DiffLine{
					DiffModify, lines[i+k].LineNumOld, lines[i+delCount+k].LineNumNew,
					lines[i+k].Content + "\x00" + lines[i+delCount+k].Content,
				})
				stats.LinesChanged++
			}
			stats.LinesRemoved -= minCount
			stats.LinesAdded -= minCount
			for k := i + minCount; k < j; k++ {
				merged = append(merged, lines[k])
			}
			i = j
		} else {
			merged = append(merged, lines[i])
			i++
		}
	}
	stats.TotalLines = len(oldLines)
	return &DiffResult{Hunks: groupHunks(merged, oldLines, newLines, context), Stats: stats}
}

func groupHunks(lines []DiffLine, _ []string, _ []string, context int) []DiffHunk {
	if len(lines) == 0 {
		return nil
	}
	changeIdxs := make([]int, 0)
	for i, l := range lines {
		if l.Type != DiffEqual {
			changeIdxs = append(changeIdxs, i)
		}
	}
	if len(changeIdxs) == 0 {
		return nil
	}

	type span struct{ start, end int }
	spans := make([]span, 0)
	cur := span{max(changeIdxs[0]-context, 0), min(changeIdxs[0]+context, len(lines)-1)}
	for _, ci := range changeIdxs[1:] {
		if ci-context <= cur.end+1 {
			cur.end = min(ci+context, len(lines)-1)
		} else {
			spans = append(spans, cur)
			cur = span{max(ci-context, 0), min(ci+context, len(lines)-1)}
		}
	}
	spans = append(spans, cur)

	hunks := make([]DiffHunk, 0, len(spans))
	for _, s := range spans {
		hunkLines := lines[s.start : s.end+1]
		var oldStart, newStart, oldCount, newCount int
		first := true
		for _, hl := range hunkLines {
			switch hl.Type {
			case DiffInsert:
				if first || newStart == 0 {
					newStart = hl.LineNumNew
					first = false
				}
				newCount++
			case DiffDelete, DiffModify:
				if first || oldStart == 0 {
					oldStart = hl.LineNumOld
					first = false
				}
				oldCount++
			default:
				if first {
					oldStart, newStart = hl.LineNumOld, hl.LineNumNew
					first = false
				}
				oldCount++
				newCount++
			}
		}
		if oldStart == 0 {
			oldStart = 1
		}
		if newStart == 0 {
			newStart = 1
		}
		hunks = append(hunks, DiffHunk{OldStart: oldStart, OldCount: oldCount, NewStart: newStart, NewCount: newCount, Lines: hunkLines})
	}
	return hunks
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	var lines []string
	cur := make([]byte, 0)
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, string(cur))
			cur = cur[:0]
		} else {
			cur = append(cur, text[i])
		}
	}
	return append(lines, string(cur))
}

func splitWords(text string) []string {
	if text == "" {
		return nil
	}
	var words []string
	cur := make([]byte, 0)
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == ',' || ch == ';' ||
			ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '[' || ch == ']' {
			if len(cur) > 0 {
				words = append(words, string(cur))
				cur = cur[:0]
			}
			words = append(words, string(ch))
		} else {
			cur = append(cur, ch)
		}
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	return words
}

func splitChars(text string) []string {
	if text == "" {
		return nil
	}
	chars := make([]string, len(text))
	for i := 0; i < len(text); i++ {
		chars[i] = string(text[i])
	}
	return chars
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
