// SPDX-License-Identifier: MIT
package workspace

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type execOutput struct {
	output string
	err    error
}

type TerminalPane struct {
	history []string
	input   string
	shown   bool
	height  int
}

func (tp *TerminalPane) runCommand(cmdStr string) tea.Cmd {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	tp.history = append(tp.history, fmt.Sprintf("$ %s", cmdStr))

	return func() tea.Msg {
		var cmd *exec.Cmd
		if len(parts) == 1 {
			cmd = exec.Command(parts[0]) //nolint:gosec
		} else {
			cmd = exec.Command(parts[0], parts[1:]...) //nolint:gosec
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		done := make(chan struct{}, 1)
		go func() {
			_ = cmd.Run()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(30 * time.Second):
			_ = cmd.Process.Kill()
			return execOutput{err: fmt.Errorf("command timed out")}
		}

		out := stdout.String()
		if stderr.Len() > 0 {
			if out != "" {
				out += "\n"
			}
			out += stderr.String()
		}

		return execOutput{output: strings.TrimSpace(out)}
	}
}

func (tp *TerminalPane) View(width int) string {
	if !tp.shown {
		return ""
	}

	var b strings.Builder

	start := 0
	if len(tp.history) > tp.height-3 {
		start = len(tp.history) - tp.height + 3
	}

	for _, line := range tp.history[start:] {
		b.WriteString(line + "\n")
	}

	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render("$ ")
	b.WriteString(prompt + tp.input + "▊")

	return b.String()
}

func (tp *TerminalPane) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case keyEnter:
		input := strings.TrimSpace(tp.input)
		tp.input = ""
		if input != "" {
			return tp.runCommand(input)
		}
	case "backspace":
		if len(tp.input) > 0 {
			tp.input = tp.input[:len(tp.input)-1]
		}
	default:
		if len(msg.Runes) == 1 && msg.Runes[0] >= ' ' {
			tp.input += string(msg.Runes)
		}
	}
	return nil
}
