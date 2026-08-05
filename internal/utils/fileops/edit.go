package fileops

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// EditOp describes a single find-and-replace operation.
type EditOp struct {
	OldText string
	NewText string
	Line    int // line number for error reporting, 0 if unknown
}

// RegexEditOp describes a regex-based find-and-replace operation.
type RegexEditOp struct {
	Pattern     string
	Replacement string
	Flags       string // e.g. "i" for case-insensitive
	Line        int
}

// EditError reports a single failed edit operation.
type EditError struct {
	OpIndex int
	Message string
	Line    int
}

func (e EditError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("edit op %d (line %d): %s", e.OpIndex, e.Line, e.Message)
	}
	return fmt.Sprintf("edit op %d: %s", e.OpIndex, e.Message)
}

// EditFile reads the file at path, applies all edits, and writes the result
// back atomically. It validates that every edit's OldText is present before
// making any changes.
func EditFile(path string, edits []EditOp) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	if errs := ValidateEdits(content, edits); len(errs) > 0 {
		return errs[0]
	}

	result, err := ApplyEdits(content, edits)
	if err != nil {
		return err
	}
	return WriteAtomic(path, []byte(result))
}

// ValidateEdits checks that every edit's OldText can be found in content.
// Returns all errors found so the caller can report them all at once.
func ValidateEdits(content string, edits []EditOp) []EditError {
	var errs []EditError
	for i, op := range edits {
		if !strings.Contains(content, op.OldText) {
			errs = append(errs, EditError{
				OpIndex: i,
				Message: fmt.Sprintf("old text not found: %.80s", op.OldText),
				Line:    op.Line,
			})
		}
	}
	return errs
}

// ApplyEdits applies all edits sequentially to content and returns the result.
// Edits are applied in order; each edit's OldText must be present.
func ApplyEdits(content string, edits []EditOp) (string, error) {
	for i, op := range edits {
		idx := strings.Index(content, op.OldText)
		if idx < 0 {
			return "", fmt.Errorf("edit op %d: old text not found", i)
		}
		// Preserve leading indentation from the matched line
		indent := extractIndent(content, idx)
		newText := preserveIndent(op.NewText, indent)
		content = content[:idx] + newText + content[idx+len(op.OldText):]
	}
	return content, nil
}

// FindAndReplace replaces the first occurrence of old with new in content.
// Returns the result, the number of replacements (0 or 1), and any error.
func FindAndReplace(content, old, new string) (string, int, error) {
	idx := strings.Index(content, old)
	if idx < 0 {
		return content, 0, nil
	}
	result := content[:idx] + new + content[idx+len(old):]
	return result, 1, nil
}

// FindAndReplaceAll replaces every occurrence of old with new in content.
func FindAndReplaceAll(content, old, new string) (string, int, error) {
	count := strings.Count(content, old)
	if count == 0 {
		return content, 0, nil
	}
	result := strings.ReplaceAll(content, old, new)
	return result, count, nil
}

// ApplyRegexEdits applies regex-based edits to content.
func ApplyRegexEdits(content string, edits []RegexEditOp) (string, int, error) {
	totalReplaced := 0
	for i, op := range edits {
		flags := op.Flags
		if !strings.Contains(flags, "g") {
			flags += "g"
		}
		pattern := "(?" + flags + ")" + op.Pattern
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", 0, fmt.Errorf("regex edit op %d: %w", i, err)
		}
		result := re.ReplaceAllString(content, op.Replacement)
		n := re.NumSubexp()
		// Approximate count: count matches before replacement
		matches := re.FindAllString(content, -1)
		totalReplaced += len(matches)
		_ = n
		content = result
	}
	return content, totalReplaced, nil
}

// extractIndent returns the leading whitespace of the line containing offset.
func extractIndent(content string, offset int) string {
	start := offset
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	indent := ""
	for i := start; i < len(content); i++ {
		ch := content[i]
		if ch == ' ' || ch == '\t' {
			indent += string(ch)
		} else {
			break
		}
	}
	return indent
}

// preserveIndent prepends indent to each line of text except the first.
func preserveIndent(text, indent string) string {
	if indent == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// EditFileRegex reads the file, applies regex edits, and writes back atomically.
func EditFileRegex(path string, edits []RegexEditOp) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	result, _, err := ApplyRegexEdits(content, edits)
	if err != nil {
		return err
	}
	return WriteAtomic(path, []byte(result))
}
