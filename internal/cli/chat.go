// SPDX-License-Identifier: MIT
package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/trace"
	"github.com/Dxrk777/Dxrk-Ai/internal/tui"
)

func RunChat(apiKey, model string) error {
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ANTHROPIC_API_KEY not set. Use --api-key or set the environment variable.")
		os.Exit(1) //nolint:revive
	}
	if model == "" {
		model = strconst.StrClaudeSonnet420250514
	}

	var tp trace.Exporter
	if os.Getenv("DXRK_TRACE") != "" {
		var err error
		tp, err = trace.NewTracerProvider("dxrk-chat")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: trace init: %v\n", err)
		} else {
			defer func() { _ = tp.Shutdown(context.Background()) }()
		}
	}

	m := tui.NewChatModel(apiKey, model, tp)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
