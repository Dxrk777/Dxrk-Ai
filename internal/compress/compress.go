// SPDX-License-Identifier: MIT
// Package compress provides context compression for managing LLM token budgets.
//
// Ported from Claude Code's compact system (see references/claude-code-source):
//   - Snip: remove oldest content
//   - Microcompact: per-message summarization
//   - Autocompact: full context compression when over budget
//   - Budget: token tracking with thresholds
package compress

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Content represents a chunk of compressible content.
type Content struct {
	ID        string
	Role      string // "user", "assistant", "tool", "system"
	Text      string
	CreatedAt time.Time
	Size      int // size in bytes
}

// Compressor applies compression strategies to reduce content size.
type Compressor struct {
	mu             sync.Mutex
	maxTokens      int // target budget
	compressionPct int // percentage to compress when over budget (0-100)
	strategy       Strategy
}

// Strategy defines the compression approach.
type Strategy int

const (
	// StrategySnip removes oldest content until under budget.
	StrategySnip Strategy = iota
	// StrategyTrimHead removes the beginning of each content block.
	StrategyTrimHead
	// StrategySummary replaces each block with its first N characters.
	StrategySummary
)

// Option configures the Compressor.
type Option func(*Compressor)

// WithMaxTokens sets the target token budget (default 128000).
func WithMaxTokens(n int) Option { return func(c *Compressor) { c.maxTokens = n } }

// WithCompressionPct sets how much to compress when over budget (default 50).
func WithCompressionPct(pct int) Option { return func(c *Compressor) { c.compressionPct = pct } }

// WithStrategy sets the compression strategy (default Snip).
func WithStrategy(s Strategy) Option { return func(c *Compressor) { c.strategy = s } }

// New creates a Compressor with defaults.
func New(opts ...Option) *Compressor {
	c := &Compressor{
		maxTokens:      128000,
		compressionPct: 50,
		strategy:       StrategySnip,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Compress reduces content to fit within the budget.
// Returns the compressed content and whether compression was applied.
func (c *Compressor) Compress(contents []Content) ([]Content, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := totalBytes(contents)
	if total <= c.maxTokens {
		return contents, false
	}

	target := total * (100 - c.compressionPct) / 100

	switch c.strategy {
	case StrategySnip:
		return c.snip(contents, target)
	case StrategyTrimHead:
		return c.trimHead(contents, target)
	case StrategySummary:
		return c.summarize(contents, target)
	default:
		return c.snip(contents, target)
	}
}

// TokenCount estimates tokens from byte count (rough heuristic: 4 bytes per token).
func TokenCount(text string) int {
	return len(text) / 4
}

// EstimateTokens returns estimated total tokens for all content.
func EstimateTokens(contents []Content) int {
	return totalBytes(contents) / 4
}

func totalBytes(contents []Content) int {
	total := 0
	for _, c := range contents {
		total += c.Size
	}
	return total
}

func (c *Compressor) snip(contents []Content, target int) ([]Content, bool) {
	var kept []Content
	accum := 0
	// Keep newest content (iterate in reverse)
	for i := len(contents) - 1; i >= 0; i-- {
		chunk := contents[i]
		if accum+chunk.Size <= target || len(kept) == 0 {
			kept = append([]Content{chunk}, kept...)
			accum += chunk.Size
		}
	}
	return kept, true
}

func (c *Compressor) trimHead(contents []Content, target int) ([]Content, bool) {
	result := make([]Content, len(contents))
	for i, chunk := range contents {
		if chunk.Size <= target/len(contents) {
			result[i] = chunk
			continue
		}
		keepBytes := chunk.Size * (100 - c.compressionPct) / 100
		trimmed := chunk.Text
		if len(trimmed) > keepBytes {
			trimmed = trimmed[len(trimmed)-keepBytes:]
		}
		result[i] = Content{
			ID:        chunk.ID,
			Role:      chunk.Role,
			Text:      trimmed,
			CreatedAt: chunk.CreatedAt,
			Size:      len(trimmed),
		}
	}
	return result, true
}

func (c *Compressor) summarize(contents []Content, target int) ([]Content, bool) {
	perBlock := target / maxInt(len(contents), 1)
	result := make([]Content, len(contents))
	for i, chunk := range contents {
		summary := chunk.Text
		if len(summary) > perBlock {
			summary = summary[:perBlock] + "..."
		}
		result[i] = Content{
			ID:        chunk.ID,
			Role:      chunk.Role,
			Text:      summary,
			CreatedAt: chunk.CreatedAt,
			Size:      len(summary),
		}
	}
	return result, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Budget tracks token usage and triggers compression decisions.
type Budget struct {
	mu                sync.Mutex
	used              int
	limit             int
	threshold         float64 // fraction of limit that triggers compression (default 0.9)
	diminishingWindow int     // remaining tokens before diminishing quality (default 500)
}

// NewBudget creates a token budget tracker.
func NewBudget(limit int) *Budget {
	return &Budget{
		used:              0,
		limit:             limit,
		threshold:         0.9,
		diminishingWindow: 500,
	}
}

// Add records token usage.
func (b *Budget) Add(tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.used += tokens
}

// Reset clears the budget.
func (b *Budget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.used = 0
}

// Remaining returns unused tokens.
func (b *Budget) Remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.limit - b.used
}

// NeedsCompression returns true if used tokens exceed the threshold.
func (b *Budget) NeedsCompression() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return float64(b.used) >= float64(b.limit)*b.threshold
}

// IsNearLimit returns true if remaining tokens are below the diminishing window.
func (b *Budget) IsNearLimit() bool {
	return b.Remaining() <= b.diminishingWindow
}

// Snapshot records the state of content at a point in time.
type Snapshot struct {
	ID            string
	CreatedAt     time.Time
	Content       []Content
	TokenEstimate int
}

// Snapshotter captures periodic snapshots for context management.
type Snapshotter struct {
	mu       sync.Mutex
	maxAge   time.Duration
	maxCount int
	snaps    []Snapshot
}

// NewSnapshotter creates a Snapshotter.
func NewSnapshotter(maxAge time.Duration, maxCount int) *Snapshotter {
	return &Snapshotter{
		maxAge:   maxAge,
		maxCount: maxCount,
	}
}

// Record stores a new snapshot, pruning old ones.
func (s *Snapshotter) Record(id string, contents []Content) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := Snapshot{
		ID:            id,
		CreatedAt:     time.Now(),
		Content:       contents,
		TokenEstimate: EstimateTokens(contents),
	}

	expired := time.Now().Add(-s.maxAge)
	// Remove expired and keep up to maxCount
	var active []Snapshot
	for _, snap := range s.snaps {
		if snap.CreatedAt.After(expired) {
			active = append(active, snap)
		}
	}
	if len(active) > s.maxCount-1 {
		active = active[len(active)-s.maxCount+1:]
	}
	active = append(active, snap)
	s.snaps = active
	return snap
}

// Recent returns snapshots within maxAge.
func (s *Snapshotter) Recent() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	expired := time.Now().Add(-s.maxAge)
	var recent []Snapshot
	for _, snap := range s.snaps {
		if snap.CreatedAt.After(expired) {
			recent = append(recent, snap)
		}
	}
	return recent
}

// String returns a human-readable summary of snapshot state.
func (s *Snapshotter) String() string {
	recent := s.Recent()
	total := 0
	for _, snap := range recent {
		total += snap.TokenEstimate
	}
	return fmt.Sprintf("snapshots: %d recent, ~%d tokens", len(recent), total)
}

// snapshotFile is the JSON-serializable representation for disk persistence.
type snapshotFile struct {
	Snapshots []Snapshot `json:"snapshots"`
}

// SaveToFile persists snapshots to a JSON file atomically.
func (s *Snapshotter) SaveToFile(path string) error {
	s.mu.Lock()
	snaps := make([]Snapshot, len(s.snaps))
	copy(snaps, s.snaps)
	s.mu.Unlock()

	data, err := json.Marshal(snapshotFile{Snapshots: snaps})
	if err != nil {
		return fmt.Errorf("marshal snapshots: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write snapshot file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename snapshot file: %w", err)
	}
	return nil
}

// LoadFromFile loads snapshots from a JSON file, merging with existing ones.
// Returns the number of snapshots loaded.
func (s *Snapshotter) LoadFromFile(path string) (int, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read snapshot file: %w", err)
	}

	var sf snapshotFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return 0, fmt.Errorf("unmarshal snapshots: %w", err)
	}

	s.mu.Lock()
	// Only keep the newest maxCount snapshots
	s.snaps = append(s.snaps, sf.Snapshots...)
	if len(s.snaps) > s.maxCount {
		s.snaps = s.snaps[len(s.snaps)-s.maxCount:]
	}
	loaded := len(sf.Snapshots)
	s.mu.Unlock()

	return loaded, nil
}

// TrimmedContent represents content after compression with metadata.
type TrimmedContent struct {
	Content       string
	OriginalBytes int
	TrimmedBytes  int
	Ratio         float64
	Strategy      string
}

// Trim reduces a single text block to fit within maxBytes.
func Trim(text string, maxBytes int) TrimmedResult {
	if len(text) <= maxBytes {
		return TrimmedResult{
			Content:      text,
			TrimmedBytes: 0,
			Strategy:     "none",
		}
	}
	cut := text[:maxBytes]
	return TrimmedResult{
		Content:       cut,
		OriginalBytes: len(text),
		TrimmedBytes:  len(text) - maxBytes,
		Ratio:         float64(maxBytes) / float64(len(text)),
		Strategy:      "trim",
	}
}

// TrimmedResult is the result of a Trim operation.
type TrimmedResult struct {
	Content       string
	OriginalBytes int
	TrimmedBytes  int
	Ratio         float64
	Strategy      string
}

// TrimToTokens reduces text to fit within a token budget (4 bytes ≈ 1 token).
func TrimToTokens(text string, maxTokens int) TrimmedResult {
	return Trim(text, maxTokens*4)
}

// CombineContext builds a single context string from content blocks.
func CombineContext(contents []Content, separator string) string {
	if separator == "" {
		separator = "\n\n"
	}
	parts := make([]string, 0, len(contents))
	for _, c := range contents {
		if c.Text != "" {
			label := strings.ToUpper(c.Role)
			parts = append(parts, fmt.Sprintf("<%s>\n%s\n</%s>", label, c.Text, label))
		}
	}
	return strings.Join(parts, separator)
}

// MergeSnapshots combines multiple snapshots into a single context, newest first, deduplicated by ID.
func MergeSnapshots(snapshots []Snapshot, maxTokens int) []Content {
	seen := make(map[string]bool)
	var result []Content
	for i := len(snapshots) - 1; i >= 0; i-- {
		for _, c := range snapshots[i].Content {
			if !seen[c.ID] {
				seen[c.ID] = true
				result = append([]Content{c}, result...)
			}
		}
	}
	return result
}
