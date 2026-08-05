// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/spf13/cobra"
)

// UsageTracker tracks token usage per model for the current session.
type UsageTracker struct {
	mu     sync.Mutex
	models map[string]*modelUsage
}

type modelUsage struct {
	InputTokens  int
	OutputTokens int
	Calls        int
}

var globalUsage = &UsageTracker{
	models: make(map[string]*modelUsage),
}

func RegisterUsageCommand(reg *Registry) {
	reg.AddCommand(&cobra.Command{
		Use:   "usage",
		Short: "Show usage metrics",
		Long:  "Display token usage breakdown by model with cost estimates.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsage()
		},
	})
}

// RecordUsage records token usage for a model.
func RecordUsage(model string, inputTokens, outputTokens int) {
	globalUsage.mu.Lock()
	defer globalUsage.mu.Unlock()

	u, ok := globalUsage.models[model]
	if !ok {
		u = &modelUsage{}
		globalUsage.models[model] = u
	}
	u.InputTokens += inputTokens
	u.OutputTokens += outputTokens
	u.Calls++
}

func runUsage() error {
	globalUsage.mu.Lock()
	defer globalUsage.mu.Unlock()

	if len(globalUsage.models) == 0 {
		fmt.Fprintln(os.Stderr, "No usage recorded yet.")
		return nil
	}

	type entry struct {
		model string
		usage *modelUsage
	}

	entries := make([]entry, 0, len(globalUsage.models))
	for m, u := range globalUsage.models {
		entries = append(entries, entry{model: m, usage: u})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].model < entries[j].model
	})

	var totalIn, totalOut, totalCalls int

	fmt.Fprintf(os.Stderr, "Usage by Model\n")
	fmt.Fprintf(os.Stderr, "──────────────\n")
	for _, e := range entries {
		totalIn += e.usage.InputTokens
		totalOut += e.usage.OutputTokens
		totalCalls += e.usage.Calls

		fmt.Fprintf(os.Stderr, "  %-30s  %6d calls  in: %s  out: %s\n",
			e.model,
			e.usage.Calls,
			formatTokens(e.usage.InputTokens),
			formatTokens(e.usage.OutputTokens),
		)
	}

	fmt.Fprintf(os.Stderr, "──────────────\n")
	fmt.Fprintf(os.Stderr, "  %-30s  %6d calls  in: %s  out: %s\n",
		"TOTAL",
		totalCalls,
		formatTokens(totalIn),
		formatTokens(totalOut),
	)

	return nil
}
