package diff

import (
	"regexp"
	"strings"
	"unicode"
)

// SymbolKind classifies what kind of symbol was renamed.
type SymbolKind string

const (
	SymVar   SymbolKind = "var"
	SymFunc  SymbolKind = "func"
	SymType  SymbolKind = "type"
	SymConst SymbolKind = "const"
)

// RenameInfo describes a detected symbol rename.
type RenameInfo struct {
	OldName string
	NewName string
	LineOld int
	LineNew int
	Type    SymbolKind
}

// MoveInfo describes a block of code that was moved.
type MoveInfo struct {
	StartOld    int
	EndOld      int
	StartNew    int
	EndNew      int
	Description string
}

// RefactorType classifies a detected refactoring operation.
type RefactorType string

const (
	RefactorExtract RefactorType = "extract"
	RefactorInline  RefactorType = "inline"
	RefactorRename  RefactorType = "rename"
	RefactorMove    RefactorType = "move"
)

// RefactorInfo describes a detected high-level refactoring.
type RefactorInfo struct {
	Type          RefactorType
	Description   string
	LinesAffected int
}

// SemanticDiff provides code-aware analysis on top of a structural diff.
type SemanticDiff struct {
	Renames   []RenameInfo
	Moves     []MoveInfo
	Refactors []RefactorInfo
}

var (
	defPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:var\s+|:\s*)(\w+)`),
		regexp.MustCompile(`func\s+(?:\([^)]*\)\s+)?(\w+)`),
		regexp.MustCompile(`type\s+(\w+)`),
		regexp.MustCompile(`const\s+(\w+)`),
	}
	defKinds = []SymbolKind{SymVar, SymFunc, SymType, SymConst}
)

// DetectRenamedLines finds symbols that were renamed between old and new text.
func DetectRenamedLines(old, new string) []RenameInfo {
	oldDefs := extractDefinitions(old)
	newDefs := extractDefinitions(new)
	var renames []RenameInfo
	for oldName, oldDef := range oldDefs {
		for newName, newDef := range newDefs {
			if oldName != newName && oldDef.typ == newDef.typ && similarity(oldName, newName) > 0.5 {
				renames = append(renames, RenameInfo{
					OldName: oldName, NewName: newName,
					LineOld: oldDef.line, LineNew: newDef.line, Type: oldDef.typ,
				})
			}
		}
	}
	return renames
}

// DetectMovedBlocks finds blocks of code that appear to have moved.
func DetectMovedBlocks(old, new string) []MoveInfo {
	oldLines := splitLines(old)
	newLines := splitLines(new)
	var moves []MoveInfo
	blockSize := 3
	for i := 0; i <= len(oldLines)-blockSize; i++ {
		block := strings.Join(oldLines[i:i+blockSize], "\n")
		for j := 0; j <= len(newLines)-blockSize; j++ {
			if j >= i-blockSize && j <= i+blockSize {
				continue
			}
			if strings.TrimSpace(block) == strings.TrimSpace(strings.Join(newLines[j:j+blockSize], "\n")) && block != "" {
				moves = append(moves, MoveInfo{
					StartOld: i + 1, EndOld: i + blockSize,
					StartNew: j + 1, EndNew: j + blockSize, Description: "block moved",
				})
				break
			}
		}
	}
	return dedupeMoves(moves)
}

// DetectRefactors identifies high-level refactoring patterns.
func DetectRefactors(old, new string) []RefactorInfo {
	var refactors []RefactorInfo
	if renames := DetectRenamedLines(old, new); len(renames) > 0 {
		parts := make([]string, len(renames))
		for i, r := range renames {
			parts[i] = r.OldName + " -> " + r.NewName
		}
		refactors = append(refactors, RefactorInfo{
			Type: RefactorRename, Description: "renamed: " + strings.Join(parts, ", "), LinesAffected: len(renames),
		})
	}
	if moves := DetectMovedBlocks(old, new); len(moves) > 0 {
		affected := 0
		for _, m := range moves {
			affected += m.EndOld - m.StartOld + 1
		}
		refactors = append(refactors, RefactorInfo{
			Type: RefactorMove, Description: "moved blocks detected", LinesAffected: affected,
		})
	}
	oldLineCount := len(splitLines(old))
	newLineCount := len(splitLines(new))
	ratio := float64(newLineCount) / float64(oldLineCount+1)
	if ratio > 1.5 && oldLineCount > 10 {
		refactors = append(refactors, RefactorInfo{
			Type: RefactorExtract, Description: "code extracted into new functions", LinesAffected: newLineCount - oldLineCount,
		})
	}
	if ratio < 0.6 && oldLineCount > 10 {
		refactors = append(refactors, RefactorInfo{
			Type: RefactorInline, Description: "code inlined, reducing line count", LinesAffected: oldLineCount - newLineCount,
		})
	}
	return refactors
}

// FilterSemanticNoise removes whitespace-only and comment-only changes from a diff.
func FilterSemanticNoise(diff *DiffResult) *DiffResult {
	if diff == nil {
		return nil
	}
	result := &DiffResult{Hunks: make([]DiffHunk, 0, len(diff.Hunks)), Stats: diff.Stats}
	for _, h := range diff.Hunks {
		filtered := make([]DiffLine, 0, len(h.Lines))
		for _, line := range h.Lines {
			if line.Type == DiffEqual {
				filtered = append(filtered, line)
				continue
			}
			if line.Type == DiffModify {
				parts := strings.SplitN(line.Content, "\x00", 2)
				if len(parts) == 2 && isWhitespaceOrCommentOnly(parts[0], parts[1]) {
					continue
				}
			}
			if !isWhitespaceOnly(line.Content) {
				filtered = append(filtered, line)
			}
		}
		if len(filtered) > 0 {
			result.Hunks = append(result.Hunks, DiffHunk{
				OldStart: h.OldStart, OldCount: h.OldCount, NewStart: h.NewStart, NewCount: h.NewCount,
				Lines: filtered, Context: h.Context,
			})
		}
	}
	return result
}

// FocusOnLogicChanges keeps only hunks containing actual logic changes.
func FocusOnLogicChanges(diff *DiffResult) *DiffResult {
	if diff == nil {
		return nil
	}
	result := &DiffResult{Hunks: make([]DiffHunk, 0, len(diff.Hunks)), Stats: diff.Stats}
	for _, h := range diff.Hunks {
		hasLogic := false
		for _, line := range h.Lines {
			if line.Type == DiffEqual {
				continue
			}
			content := line.Content
			if line.Type == DiffModify {
				parts := strings.SplitN(content, "\x00", 2)
				if len(parts) == 2 && !isCommentOnly(parts[0]) && !isCommentOnly(parts[1]) {
					hasLogic = true
					break
				}
			} else if !isCommentOnly(content) && !isWhitespaceOnly(content) {
				hasLogic = true
				break
			}
		}
		if hasLogic {
			result.Hunks = append(result.Hunks, h)
		}
	}
	return result
}

// --- helpers ---

type symbolDef struct {
	typ  SymbolKind
	line int
}

func extractDefinitions(text string) map[string]symbolDef {
	defs := make(map[string]symbolDef)
	for i, line := range splitLines(text) {
		trimmed := strings.TrimSpace(line)
		for pi, pat := range defPatterns {
			if m := pat.FindStringSubmatch(trimmed); m != nil {
				defs[m[1]] = symbolDef{typ: defKinds[pi], line: i + 1}
			}
		}
	}
	return defs
}

func similarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	set := make(map[rune]bool, len(a))
	for _, r := range a {
		set[r] = true
	}
	match := 0
	for _, r := range b {
		if set[r] {
			match++
		}
	}
	al, bl := len([]rune(a)), len([]rune(b))
	if al > bl {
		return float64(match) / float64(al)
	}
	return float64(match) / float64(bl)
}

func dedupeMoves(moves []MoveInfo) []MoveInfo {
	if len(moves) <= 1 {
		return moves
	}
	seen := make(map[string]bool)
	var result []MoveInfo
	for _, m := range moves {
		key := itoa(m.StartOld) + ":" + itoa(m.StartNew)
		if !seen[key] {
			seen[key] = true
			result = append(result, m)
		}
	}
	return result
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func isWhitespaceOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func isCommentOnly(s string) bool {
	t := strings.TrimSpace(s)
	return t != "" && (strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") ||
		strings.HasPrefix(t, "*") || strings.HasPrefix(t, "#"))
}

func isWhitespaceOrCommentOnly(old, new string) bool {
	return (isWhitespaceOnly(old) && isWhitespaceOnly(new)) ||
		(isCommentOnly(old) && isCommentOnly(new))
}
