// SPDX-License-Identifier: MIT
package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/query"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
	"github.com/Dxrk777/Dxrk-Ai/internal/trace"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	sidebarStyle = lipgloss.NewStyle().
			Width(30).
			Border(lipgloss.NormalBorder()).
			BorderRight(true).
			Padding(0, 1)

	chatStyle = lipgloss.NewStyle().
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Height(1).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	symlinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))
)

type pane int

const (
	paneFileTree pane = iota
	paneChat
	paneTerminal
)

type Model struct {
	root       string
	files      []fileEntry
	cursor     int
	expanded   map[string]bool
	activePane pane

	chatInput    string
	chatHistory  []chatMessage
	chatViewport viewport.Model

	chatProvider query.Provider
	systemPrompt string
	terminal     TerminalPane
	showTerminal bool

	width  int
	height int

	ctx    context.Context
	cancel context.CancelFunc

	toolRegistry *tools.Registry
	tp           trace.Exporter
}

type fileEntry struct {
	name      string
	path      string
	isDir     bool
	isSymlink bool
	children  []fileEntry
	depth     int
}

type chatMessage struct {
	role    string
	content string
}

type WorkspaceOption func(*Model)

func WithChatProvider(p query.Provider) WorkspaceOption {
	return func(m *Model) { m.chatProvider = p }
}

func WithSystemPrompt(prompt string) WorkspaceOption {
	return func(m *Model) { m.systemPrompt = prompt }
}

func New(root string, toolRegistry *tools.Registry, tp trace.Exporter, opts ...WorkspaceOption) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	vp := viewport.New(0, 0)
	vp.YPosition = 0

	m := &Model{
		root:         root,
		expanded:     make(map[string]bool),
		activePane:   paneFileTree,
		chatViewport: vp,
		ctx:          ctx,
		cancel:       cancel,
		toolRegistry: toolRegistry,
		tp:           tp,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.loadFiles("", 0)
	return m
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chatMsg:
		if msg.err != nil {
			m.chatHistory = append(m.chatHistory, chatMessage{role: roleSystem, content: "Error: " + msg.err.Error()})
		} else {
			m.chatHistory = append(m.chatHistory, chatMessage{role: strconst.StrAssistant, content: msg.text})
		}
		m.chatViewport.GotoBottom()

	case execOutput:
		m.terminal.history = append(m.terminal.history, msg.output)

	case tea.KeyMsg:
		if m.activePane == paneChat {
			return m, m.handleChatInput(msg)
		}
		if m.activePane == paneTerminal {
			return m, m.terminal.handleKey(msg)
		}
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - 1
		sidebarW := 30
		chatW := m.width - sidebarW - 2
		m.chatViewport.Width = chatW
		m.chatViewport.Height = m.height - 2
		return m, nil
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancel()
		return m, tea.Quit

	case "tab":
		switch m.activePane {
		case paneFileTree:
			m.activePane = paneChat
		case paneChat:
			m.activePane = paneTerminal
		case paneTerminal:
			m.activePane = paneFileTree
		}

	case "up", "k":
		if m.activePane == paneFileTree {
			m.cursorUp()
		}

	case "down", "j":
		if m.activePane == paneFileTree {
			m.cursorDown()
		}

	case keyEnter:
		if m.activePane == paneFileTree {
			m.toggleExpand()
		}

	case "l", "right":
		if m.activePane == paneFileTree {
			m.expandCurrent()
		}

	case "h", "left":
		if m.activePane == paneFileTree {
			m.collapseCurrent()
		}

	case "r":
		if m.activePane == paneFileTree {
			m.files = nil
			m.loadFiles("", 0)
		}

	case "ctrl+t":
		m.showTerminal = !m.showTerminal
		if m.showTerminal {
			m.terminal.shown = true
			m.terminal.height = 10
		} else {
			m.terminal.shown = false
		}
	}

	return m, nil
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	sidebarW := 30
	sidebar := m.renderSidebar(sidebarW)

	chatContent := m.renderChat()

	chatW := m.width - sidebarW - 2
	chatH := m.height - 2
	if m.showTerminal {
		chatH = m.height - 2 - m.terminal.height
	}
	chatPanel := chatStyle.Width(chatW).Height(chatH).Render(chatContent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, chatPanel)

	if m.showTerminal {
		terminalContent := m.terminal.View(chatW)
		termPanel := lipgloss.NewStyle().
			Width(chatW).
			Height(m.terminal.height).
			Border(lipgloss.NormalBorder()).
			BorderTop(true).
			Render(terminalContent)
		body = lipgloss.JoinVertical(lipgloss.Left, body, termPanel)
	}

	statusBar := m.renderStatus()

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

func (m *Model) renderSidebar(width int) string {
	title := fmt.Sprintf("Workspace: %s", filepath.Base(m.root))
	titleBar := lipgloss.NewStyle().Width(width).Padding(0, 1).
		Foreground(lipgloss.Color("86")).Render(title)

	entries := m.renderEntries(m.files, 0, 0)
	content := lipgloss.JoinVertical(lipgloss.Left, entries...)

	return sidebarStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleBar, content),
	)
}

func (m *Model) renderEntries(entries []fileEntry, depth int, globalIdx int) []string {
	var lines []string
	for _, e := range entries {
		prefix := strings.Repeat("  ", e.depth)
		var icon string
		cursor := " "
		if globalIdx == m.cursor && m.activePane == paneFileTree {
			cursor = "▸"
		}
		switch {
		case e.isDir:
			if m.expanded[e.path] {
				icon = "▼"
			} else {
				icon = "▶"
			}
		case e.isSymlink:
			icon = "→"
		default:
			icon = " "
		}

		var name string
		switch {
		case e.isDir:
			name = dirStyle.Render(e.name + "/")
		case e.isSymlink:
			name = symlinkStyle.Render(e.name)
		default:
			name = fileStyle.Render(e.name)
		}

		line := prefix + cursor + " " + icon + " " + name
		if cursor == "▸" {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
		globalIdx++

		if e.isDir && m.expanded[e.path] {
			subLines := m.renderEntries(e.children, depth+1, globalIdx)
			lines = append(lines, subLines...)
			globalIdx += len(subLines)
		}
	}
	return lines
}

func (m *Model) renderChat() string {
	var b strings.Builder
	b.WriteString(activeTabStyle.Render(" Chat ") + "\n\n")
	for _, msg := range m.chatHistory {
		role := strings.ToUpper(msg.role[:1]) + msg.role[1:]
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(role+":") + " ")
		b.WriteString(msg.content + "\n\n")
	}
	if m.activePane == paneChat {
		b.WriteString("\n> " + m.chatInput + "▊")
	}
	return b.String()
}

func (m *Model) renderStatus() string {
	var b strings.Builder
	paneName := "filetree"
	switch m.activePane {
	case paneChat:
		paneName = "chat"
	case paneTerminal:
		paneName = "terminal"
	}
	fmt.Fprintf(&b, " [%s] tab:switch | arrows:navigate | enter:expand | r:refresh | ^T:terminal | q:quit", paneName)
	return statusStyle.Width(m.width).Render(b.String())
}
