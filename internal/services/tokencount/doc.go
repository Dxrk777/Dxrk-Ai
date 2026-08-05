// Package tokencount provides token estimation and counting services
// for message budgets, context windows, and conversation management.
//
// It uses heuristic-based estimation (avg 4 chars per token for English,
// ~2 chars per token for CJK, code blocks use ~3 chars per token).
// The service is self-contained with no external dependencies beyond stdlib.
package tokencount
