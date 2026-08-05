package diff

import (
	"errors"
	"fmt"
	"strings"
)

// Errors returned by patch operations.
var (
	ErrPatchInvalid    = errors.New("patch is invalid")
	ErrContextMismatch = errors.New("context mismatch")
	ErrPatchConflict   = errors.New("patch conflict")
	ErrPatchEmpty      = errors.New("patch has no hunks")
)

// Patch represents a set of changes that can be applied to text.
type Patch struct {
	Description string
	Hunks       []PatchHunk
	Applied     bool
}

// PatchHunk represents a single change region within a patch.
type PatchHunk struct {
	OldStart int
	OldLines []string
	NewLines []string
}

// CreatePatch builds a Patch from a DiffResult.
func CreatePatch(diff *DiffResult) *Patch {
	if diff == nil {
		return &Patch{}
	}
	p := &Patch{
		Description: fmt.Sprintf("%d hunks, +%d -%d ~%d",
			len(diff.Hunks), diff.Stats.LinesAdded, diff.Stats.LinesRemoved, diff.Stats.LinesChanged),
		Hunks: make([]PatchHunk, 0, len(diff.Hunks)),
	}

	for _, h := range diff.Hunks {
		ph := PatchHunk{OldStart: h.OldStart}
		for _, line := range h.Lines {
			switch line.Type {
			case DiffEqual:
				ph.OldLines = append(ph.OldLines, line.Content)
				ph.NewLines = append(ph.NewLines, line.Content)
			case DiffDelete:
				ph.OldLines = append(ph.OldLines, line.Content)
			case DiffInsert:
				ph.NewLines = append(ph.NewLines, line.Content)
			case DiffModify:
				parts := strings.SplitN(line.Content, "\x00", 2)
				ph.OldLines = append(ph.OldLines, parts[0])
				if len(parts) > 1 {
					ph.NewLines = append(ph.NewLines, parts[1])
				}
			}
		}
		p.Hunks = append(p.Hunks, ph)
	}
	return p
}

// ApplyPatch applies a patch to the original text and returns the result.
func ApplyPatch(original string, patch *Patch) (string, error) {
	if err := ValidatePatch(original, patch); err != nil {
		return "", err
	}

	lines := splitLines(original)
	result := make([]string, 0, len(lines))
	cursor := 0

	for _, hunk := range patch.Hunks {
		start := hunk.OldStart - 1
		if start < 0 {
			start = 0
		}

		// Copy lines before this hunk.
		for cursor < start && cursor < len(lines) {
			result = append(result, lines[cursor])
			cursor++
		}

		// Verify context lines match.
		for i, ctx := range hunk.OldLines {
			idx := start + i
			if idx < len(lines) && lines[idx] != ctx {
				// Fuzzy: try to find the context nearby.
				found := false
				for offset := -3; offset <= 3; offset++ {
					alt := idx + offset
					if alt >= 0 && alt < len(lines) && lines[alt] == ctx {
						found = true
						break
					}
				}
				if !found {
					return "", fmt.Errorf("%w: expected %q at line %d", ErrContextMismatch, ctx, idx+1)
				}
			}
		}

		// Replace old lines with new lines.
		result = append(result, hunk.NewLines...)
		cursor = start + len(hunk.OldLines)
	}

	// Copy remaining lines.
	for cursor < len(lines) {
		result = append(result, lines[cursor])
		cursor++
	}

	patch.Applied = true
	return strings.Join(result, "\n"), nil
}

// RevertPatch undoes a previously applied patch.
func RevertPatch(modified string, patch *Patch) (string, error) {
	if !patch.Applied {
		return modified, nil
	}

	// Build reverse patch: swap old and new lines.
	revPatch := &Patch{
		Description: "revert: " + patch.Description,
		Hunks:       make([]PatchHunk, 0, len(patch.Hunks)),
	}
	for _, h := range patch.Hunks {
		revPatch.Hunks = append(revPatch.Hunks, PatchHunk{
			OldStart: h.OldStart,
			OldLines: h.NewLines,
			NewLines: h.OldLines,
		})
	}

	result, err := ApplyPatch(modified, revPatch)
	if err != nil {
		return "", fmt.Errorf("revert failed: %w", err)
	}
	patch.Applied = false
	return result, nil
}

// ValidatePatch checks whether a patch can be applied to the original text.
func ValidatePatch(original string, patch *Patch) error {
	if patch == nil || len(patch.Hunks) == 0 {
		return ErrPatchEmpty
	}

	lines := splitLines(original)
	for i, hunk := range patch.Hunks {
		start := hunk.OldStart - 1
		if start < 0 {
			return fmt.Errorf("%w: hunk %d has invalid start line %d", ErrPatchInvalid, i+1, hunk.OldStart)
		}
		end := start + len(hunk.OldLines)
		if end > len(lines) {
			return fmt.Errorf("%w: hunk %d extends beyond file (line %d > %d)",
				ErrPatchInvalid, i+1, end, len(lines))
		}

		// Check context lines.
		for j, ctx := range hunk.OldLines {
			idx := start + j
			if idx < len(lines) && lines[idx] != ctx {
				// Allow fuzzy matching.
				found := false
				for offset := -3; offset <= 3; offset++ {
					alt := idx + offset
					if alt >= 0 && alt < len(lines) && lines[alt] == ctx {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("%w: hunk %d, line %d: expected %q, got %q",
						ErrContextMismatch, i+1, idx+1, ctx, lines[idx])
				}
			}
		}
	}
	return nil
}

// MergePatches combines two non-overlapping patches into one.
func MergePatches(a, b *Patch) (*Patch, error) {
	if a == nil || b == nil {
		return nil, ErrPatchEmpty
	}

	// Check for overlapping hunks.
	for _, ha := range a.Hunks {
		for _, hb := range b.Hunks {
			aEnd := ha.OldStart + len(ha.OldLines)
			bEnd := hb.OldStart + len(hb.OldLines)
			if ha.OldStart < bEnd && hb.OldStart < aEnd {
				return nil, fmt.Errorf("%w: hunks overlap at lines %d-%d",
					ErrPatchConflict, max(ha.OldStart, hb.OldStart), min(aEnd, bEnd))
			}
		}
	}

	merged := &Patch{
		Description: a.Description + " + " + b.Description,
		Hunks:       make([]PatchHunk, 0, len(a.Hunks)+len(b.Hunks)),
	}
	merged.Hunks = append(merged.Hunks, a.Hunks...)
	merged.Hunks = append(merged.Hunks, b.Hunks...)

	// Sort by start line.
	for i := 0; i < len(merged.Hunks); i++ {
		for j := i + 1; j < len(merged.Hunks); j++ {
			if merged.Hunks[j].OldStart < merged.Hunks[i].OldStart {
				merged.Hunks[i], merged.Hunks[j] = merged.Hunks[j], merged.Hunks[i]
			}
		}
	}

	return merged, nil
}
