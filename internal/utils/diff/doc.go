// Package diff provides structured diff computation, formatting, and patch
// utilities for file edits, tool outputs, and conversation changes.
//
// It supports line-level, word-level, and character-level diffs using an
// LCS (Longest Common Subsequence) algorithm. Output formats include
// unified, context, side-by-side, compact, Markdown, HTML, and JSON.
//
// The patch sub-system can create, apply, revert, and merge patches with
// fuzzy matching and offset adjustments. File-level operations handle
// whole-file and directory-wide comparisons. A semantic diff layer
// detects renames, moves, and high-level refactors while filtering
// whitespace and comment-only noise.
package diff
