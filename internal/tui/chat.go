// SPDX-License-Identifier: MIT
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Dxrk777/Dxrk-Ai/internal/agents"
	"github.com/Dxrk777/Dxrk-Ai/internal/compress"
	"github.com/Dxrk777/Dxrk-Ai/internal/query"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
	dxrktools "github.com/Dxrk777/Dxrk-Ai/internal/tools/dxrk"
	"github.com/Dxrk777/Dxrk-Ai/internal/trace"
	"github.com/Dxrk777/Dxrk-Ai/internal/tui/styles"
)

const roleAssistant = "assistant"

type chatMsg struct {
	text string
	err  error
}

type chatMessage struct {
	role    string
	content string
}

type ChatModel struct {
	messages  []chatMessage
	input     textarea.Model
	width     int
	height    int
	provider  query.Provider
	toolReg   *tools.Registry
	isLoading bool
	err       error
	quitting  bool
	tp        trace.Exporter
}

func NewChatModel(apiKey, modelName string, tp trace.Exporter) ChatModel {
	ta := textarea.New()
	ta.Placeholder = "Ask anything..."
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.CharLimit = 10000
	ta.ShowLineNumbers = false

	reg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err == nil {
		_ = dxrktools.RegisterAll(reg, agentReg)
	}

	provider := query.NewAnthropicProvider(apiKey, modelName)

	return ChatModel{
		input:    ta,
		provider: provider,
		toolReg:  reg,
		tp:       tp,
	}
}

func (m ChatModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)
		return m, nil

	case chatMsg:
		m.isLoading = false
		if msg.err != nil {
			m.err = msg.err
			m.messages = append(m.messages, chatMessage{
				role: roleAssistant, content: fmt.Sprintf("Error: %v", msg.err),
			})
		} else {
			m.messages = append(m.messages, chatMessage{
				role: roleAssistant, content: msg.text,
			})
		}
		return m, nil

	case tea.KeyMsg:
		if m.quitting {
			return m, tea.Quit
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.isLoading {
				return m, nil
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}

			m.messages = append(m.messages, chatMessage{role: "user", content: text})
			m.input.Reset()
			m.isLoading = true
			m.err = nil

			msgs := m.toQueryMessages()
			loopOpts := []query.Option{
				query.WithMaxTurns(10),
				query.WithCompressor(compress.New(compress.WithStrategy(compress.StrategySnip))),
				query.WithBudget(compress.NewBudget(50000)),
			}
			if m.tp != nil {
				loopOpts = append(loopOpts, query.WithTracer(m.tp))
			}
			loop := query.New(m.provider, m.toolReg, loopOpts...)
			ctx := context.Background()

			return m, func() tea.Msg {
				result, err := loop.Run(ctx, msgs)
				if err != nil {
					return chatMsg{err: err}
				}
				return chatMsg{text: result.FinalText}
			}

		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

	default:
		return m, nil
	}
}

func (m ChatModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	header := styles.TitleStyle.Render("Chat Mode")
	b.WriteString(header)
	b.WriteString("\n\n")

	msgAreaH := m.height - 7
	if msgAreaH < 5 {
		msgAreaH = 5
	}

	var msgsStr strings.Builder
	for _, msg := range m.messages {
		label := "You"
		style := styles.SelectedStyle
		if msg.role == roleAssistant {
			label = "AI"
			style = styles.SuccessStyle
		}
		msgsStr.WriteString(style.Render(label + ":"))
		msgsStr.WriteString("\n")

		lines := strings.Split(msg.content, "\n")
		for _, l := range lines {
			msgsStr.WriteString("  " + l + "\n")
		}
		msgsStr.WriteString("\n")
	}

	if m.err != nil {
		msgsStr.WriteString(styles.ErrorStyle.Render("Error: "+m.err.Error()) + "\n\n")
	}

	if m.isLoading {
		msgsStr.WriteString(styles.SubtextStyle.Render("Thinking...") + "\n")
	}

	msgContent := msgsStr.String()
	msgLines := strings.Split(msgContent, "\n")
	if len(msgLines) > msgAreaH {
		msgLines = msgLines[len(msgLines)-msgAreaH:]
		msgContent = strings.Join(msgLines, "\n")
	}
	b.WriteString(lipgloss.NewStyle().Height(msgAreaH).Render(msgContent))

	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("enter: send • ctrl+c: quit"))

	return styles.FrameStyle.Render(b.String())
}

func (m ChatModel) toQueryMessages() []query.Message {
	qmsgs := make([]query.Message, 0, len(m.messages))
	for _, msg := range m.messages {
		role := query.RoleUser
		if msg.role == roleAssistant {
			role = query.RoleAssistant
		}
		qmsgs = append(qmsgs, query.Message{
			Role:    role,
			Content: msg.content,
		})
	}
	return qmsgs
}
