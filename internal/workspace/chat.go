// SPDX-License-Identifier: MIT
package workspace

import (
	"context"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/query"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"

	tea "github.com/charmbracelet/bubbletea"
)

type chatMsg struct {
	text string
	err  error
}

func (m *Model) sendChatMessage(input string) tea.Cmd {
	msg := chatMessage{role: "user", content: input}
	m.chatHistory = append(m.chatHistory, msg)
	m.chatInput = ""
	m.chatViewport.GotoBottom()

	if m.chatProvider == nil {
		m.chatHistory = append(m.chatHistory, chatMessage{
			role:    roleSystem,
			content: "No chat provider configured. Use WithChatProvider when creating the workspace.",
		})
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 120*time.Second)
		defer cancel()

		messages := m.buildChatMessages(input)
		resp, err := m.chatProvider.Generate(ctx, messages, nil)
		if err != nil {
			return chatMsg{err: err}
		}
		return chatMsg{text: resp.Text}
	}
}

const (
	roleSystem = strconst.StrSystem
	keyEnter   = "enter"
)

func (m *Model) buildChatMessages(_ string) []query.Message {
	var msgs []query.Message
	if m.systemPrompt != "" {
		msgs = append(msgs, query.Message{
			Role:    query.RoleSystem,
			Content: m.systemPrompt,
		})
	}
	for _, h := range m.chatHistory {
		role := query.RoleUser
		if h.role == strconst.StrAssistant || h.role == strconst.StrSystem {
			role = query.RoleAssistant
		}
		msgs = append(msgs, query.Message{
			Role:    role,
			Content: h.content,
		})
	}
	return msgs
}

func (m *Model) handleChatInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case keyEnter:
		input := strings.TrimSpace(m.chatInput)
		if input == "" {
			return nil
		}
		return m.sendChatMessage(input)

	case "backspace":
		if len(m.chatInput) > 0 {
			m.chatInput = m.chatInput[:len(m.chatInput)-1]
		}

	default:
		if len(msg.Runes) == 1 && msg.Runes[0] >= ' ' {
			m.chatInput += string(msg.Runes)
		}
	}
	return nil
}
