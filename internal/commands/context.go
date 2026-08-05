// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

const (
	// DefaultContextLimit is the default max token budget for the context window.
	DefaultContextLimit = 128_000
	// CompactThresholdPct triggers compaction when context usage exceeds this percentage.
	CompactThresholdPct = 80
)

// ContextManager tracks context window usage.
type ContextManager struct {
	mu          sync.Mutex
	maxTokens   int
	usedTokens  int
	compactions int
}

var globalContext = &ContextManager{
	maxTokens: DefaultContextLimit,
}

func RegisterContextCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "context [show|compact|reset]",
		Short: "Show and manage context window usage",
		Long:  "Display context window usage, remaining capacity, and manage compaction.",
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "show"
			if len(args) > 0 {
				action = args[0]
			}
			switch action {
			case "show":
				return runContextShow()
			case "compact":
				return runContextCompact()
			case "reset":
				return runContextReset()
			default:
				return fmt.Errorf("unknown context action %q (use show, compact, or reset)", action)
			}
		},
	}
	reg.AddCommand(cmd)
}

func runContextShow() error {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()

	used := globalContext.usedTokens
	max := globalContext.maxTokens
	remaining := max - used
	pct := float64(used) / float64(max) * 100

	bar := renderBar(pct, 30)

	fmt.Fprintf(os.Stderr, "Context Window\n")
	fmt.Fprintf(os.Stderr, "──────────────\n")
	fmt.Fprintf(os.Stderr, "  Used:        %s / %s (%.1f%%)\n", formatTokens(used), formatTokens(max), pct)
	fmt.Fprintf(os.Stderr, "  Remaining:   %s\n", formatTokens(remaining))
	fmt.Fprintf(os.Stderr, "  Compactions: %d\n", globalContext.compactions)
	fmt.Fprintf(os.Stderr, "\n  %s\n", bar)

	if pct >= float64(CompactThresholdPct) {
		fmt.Fprintf(os.Stderr, "\n  ⚠ Context usage exceeds %d%% — consider running 'dxrk context compact'\n", CompactThresholdPct)
	}

	return nil
}

func runContextCompact() error {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()

	before := globalContext.usedTokens
	globalContext.usedTokens = globalContext.usedTokens * 60 / 100
	globalContext.compactions++

	fmt.Fprintf(os.Stderr, "Context compacted: %s → %s (saved %s)\n",
		formatTokens(before),
		formatTokens(globalContext.usedTokens),
		formatTokens(before-globalContext.usedTokens),
	)
	return nil
}

func runContextReset() error {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()

	globalContext.usedTokens = 0
	globalContext.compactions = 0

	fmt.Fprintf(os.Stderr, "Context reset to 0 tokens.\n")
	return nil
}

// AddTokens records token usage into the context window.
func AddTokens(n int) {
	globalContext.mu.Lock()
	globalContext.usedTokens += n
	globalContext.mu.Unlock()
}

// NeedsCompact reports whether context usage exceeds the compaction threshold.
func NeedsCompact() bool {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()
	pct := float64(globalContext.usedTokens) / float64(globalContext.maxTokens) * 100
	return pct >= float64(CompactThresholdPct)
}

// SetContextLimit overrides the maximum token budget.
func SetContextLimit(tokens int) {
	globalContext.mu.Lock()
	globalContext.maxTokens = tokens
	globalContext.mu.Unlock()
}

func renderBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	return bar
}
