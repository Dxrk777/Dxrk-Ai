// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// SessionStats holds runtime statistics for a Dxrk session.
type SessionStats struct {
	StartedAt  time.Time
	Messages   int
	ToolCalls  int
	TokensUsed int
	Model      string
}

var currentSession = &SessionStats{
	StartedAt: time.Now(),
}

func RegisterStatsCommand(reg *Registry) {
	reg.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Show session statistics",
		Long:  "Display tokens, messages, tools used, and session duration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStats()
		},
	})
}

func runStats() error {
	s := currentSession
	elapsed := time.Since(s.StartedAt).Round(time.Second)

	fmt.Fprintf(os.Stderr, "Session Statistics\n")
	fmt.Fprintf(os.Stderr, "──────────────────\n")
	fmt.Fprintf(os.Stderr, "  Started:      %s\n", s.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(os.Stderr, "  Duration:     %s\n", elapsed)
	fmt.Fprintf(os.Stderr, "  Model:        %s\n", modelOrNone(s.Model))
	fmt.Fprintf(os.Stderr, "  Messages:     %d\n", s.Messages)
	fmt.Fprintf(os.Stderr, "  Tool calls:   %d\n", s.ToolCalls)
	fmt.Fprintf(os.Stderr, "  Tokens used:  %s\n", formatTokens(s.TokensUsed))
	return nil
}

// RecordMessage increments the message counter.
func RecordMessage() {
	currentSession.Messages++
}

// RecordToolCall increments the tool call counter.
func RecordToolCall() {
	currentSession.ToolCalls++
}

// RecordTokens adds tokens to the running total.
func RecordTokens(n int) {
	currentSession.TokensUsed += n
}

// SetModel sets the current model name for display.
func SetModel(model string) {
	currentSession.Model = model
}

func modelOrNone(m string) string {
	if m == "" {
		return "(none)"
	}
	return m
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
