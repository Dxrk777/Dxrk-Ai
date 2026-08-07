// SPDX-License-Identifier: MIT
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/compress"
	"github.com/Dxrk777/Dxrk/internal/query"
	"github.com/Dxrk777/Dxrk/internal/tools"
	dxrktools "github.com/Dxrk777/Dxrk/internal/tools/dxrk"
	"github.com/Dxrk777/Dxrk/internal/tools/filetools"
)

// QueryFlags holds parsed flags for the query command.
type QueryFlags struct {
	Message  string
	Model    string
	MaxTurns int
	APIKey   string
	System   string
	Project  string
}

// ParseQueryFlags parses flags for `dxrk query`.`dxrk query`.
func ParseQueryFlags(args []string) (QueryFlags, error) {
	var opts QueryFlags
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(flagDiscard{})
	fs.StringVar(&opts.Message, "message", "", "User message to send")
	fs.StringVar(&opts.Message, "m", "", "User message to send (shorthand)")
	fs.StringVar(&opts.Model, "model", "claude-sonnet-4-20250514", "Anthropic model to use")
	fs.StringVar(&opts.APIKey, "api-key", "", "Anthropic API key (defaults to ANTHROPIC_API_KEY env)")
	fs.StringVar(&opts.System, "system", "You are a helpful assistant with access to Dxrk system tools.", "System prompt")
	fs.IntVar(&opts.MaxTurns, "max-turns", 10, "Maximum query loop turns")
	fs.StringVar(&opts.Project, "project", "", "Project name for Dxrk Memory persistence (optional)")

	if err := fs.Parse(args); err != nil {
		return QueryFlags{}, err
	}

	// If no message flag, use positional args as message
	if opts.Message == "" {
		opts.Message = strings.Join(fs.Args(), " ")
	}

	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	return opts, nil
}

// RunQuery executes a single query loop.
func RunQuery(flags QueryFlags) error {
	if flags.Message == "" {
		return fmt.Errorf("message is required: use --message or pass a positional argument")
	}
	if flags.APIKey == "" {
		return fmt.Errorf("anthropic API key required: set ANTHROPIC_API_KEY or pass --api-key")
	}

	// Create registries
	toolReg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		return fmt.Errorf("create agent registry: %w", err)
	}
	if err := dxrktools.RegisterAll(toolReg, agentReg); err != nil {
		return fmt.Errorf("register tools: %w", err)
	}
	if err := filetools.RegisterAll(toolReg); err != nil {
		return fmt.Errorf("register filetools: %w", err)
	}

	// Create provider
	provider := query.NewAnthropicProvider(flags.APIKey, flags.Model)

	// Create compressor + budget
	comp := compress.New(
		compress.WithMaxTokens(128000),
		compress.WithCompressionPct(50),
		compress.WithStrategy(compress.StrategySnip),
	)
	budget := compress.NewBudget(128000)

	var opts []query.Option
	opts = append(opts, query.WithMaxTurns(flags.MaxTurns))
	opts = append(opts, query.WithCompressor(comp))
	opts = append(opts, query.WithBudget(budget))
	opts = append(opts, query.WithTurnCallback(func(turn, msgs, tools int) {
		fmt.Fprintf(os.Stderr, "\r🔄 Turn %d (%d messages, %d tool calls)", turn, msgs, tools)
	}))

	// Connect to DxrkMemory for cross-session persistence (optional)
	if flags.Project != "" {
		dxrkMemory, err := query.NewDxrkMemoryBackend(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Dxrk Memory not available: %v\n", err)
		}
		if dxrkMemory != nil {
			fmt.Fprintf(os.Stderr, "📝 Dxrk Memory persistence enabled for project %q\n", flags.Project)
			opts = append(opts, query.WithPersistence(dxrkMemory))
			defer func() { _ = dxrkMemory.Close() }()
		}
	}

	loop := query.New(provider, toolReg, opts...)

	start := time.Now()
	result, err := loop.Run(context.Background(), []query.Message{
		{Role: query.RoleSystem, Content: flags.System, CreatedAt: time.Now()},
		{Role: query.RoleUser, Content: flags.Message, CreatedAt: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("query loop: %w", err)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "---")
	fmt.Fprintf(os.Stderr, "Turns: %d | Tool calls: %d | Duration: %v | Stop: %s\n",
		result.Turns, result.ToolCalls, elapsed, result.StopReason)

	fmt.Println(result.FinalText)
	return nil
}

type flagDiscard struct{}

func (flagDiscard) Write(p []byte) (int, error) { return len(p), nil }
