// SPDX-License-Identifier: MIT
package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/query"
	"github.com/Dxrk777/Dxrk/internal/tools"

	tea "github.com/charmbracelet/bubbletea"
)

type mockProvider struct{}

func (m mockProvider) Generate(ctx context.Context, msgs []query.Message, toolSchemas []query.ToolSchema) (query.Response, error) {
	return query.Response{
		Text:  "mock response",
		Usage: query.Usage{InputTokens: 10, OutputTokens: 20},
	}, nil
}

func setupWorkspace(t *testing.T) (*Model, string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0o644)
	os.WriteFile(filepath.Join(dir, "file2.go"), []byte("package main"), 0o644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "subdir", "nested.txt"), []byte("nested"), 0o644)

	m := New(dir, tools.New(), nil, WithChatProvider(mockProvider{}))
	return m, dir
}

func TestNewWorkspace(t *testing.T) {
	m, _ := setupWorkspace(t)
	if m == nil {
		t.Fatal("expected non-nil Model")
	}
	if m.root == "" {
		t.Fatal("expected non-empty root")
	}
	if m.chatProvider == nil {
		t.Fatal("expected chat provider to be set")
	}
}

func TestNewWorkspaceWithoutChatProvider(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, tools.New(), nil)
	if m.chatProvider != nil {
		t.Fatal("expected nil chat provider when not configured")
	}
}

func TestInitReturnsNil(t *testing.T) {
	m, _ := setupWorkspace(t)
	cmd := m.Init()
	if cmd != nil {
		t.Fatal("expected nil command from Init")
	}
}

func TestUpdateHandlesWindowSize(t *testing.T) {
	m, _ := setupWorkspace(t)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	_, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("expected nil command from WindowSizeMsg")
	}
	if m.width != 120 {
		t.Fatalf("expected width 120, got %d", m.width)
	}
	if m.height != 39 {
		t.Fatalf("expected height 39, got %d", m.height)
	}
}

func TestUpdateHandlesChatMsg(t *testing.T) {
	m, _ := setupWorkspace(t)

	_, cmd := m.Update(chatMsg{text: "hello from assistant"})
	if len(m.chatHistory) != 1 {
		t.Fatalf("expected 1 chat message, got %d", len(m.chatHistory))
	}
	if m.chatHistory[0].content != "hello from assistant" {
		t.Fatalf("unexpected content: %q", m.chatHistory[0].content)
	}
	if m.chatHistory[0].role != "assistant" {
		t.Fatalf("expected role 'assistant', got %q", m.chatHistory[0].role)
	}
	if cmd != nil {
		t.Fatal("expected nil command from chatMsg")
	}
}

func TestUpdateHandlesChatMsgError(t *testing.T) {
	m, _ := setupWorkspace(t)

	_, cmd := m.Update(chatMsg{err: assert("something went wrong")})
	if len(m.chatHistory) != 1 {
		t.Fatalf("expected 1 chat message, got %d", len(m.chatHistory))
	}
	if m.chatHistory[0].role != "system" {
		t.Fatalf("expected role 'system' for error, got %q", m.chatHistory[0].role)
	}
	if cmd != nil {
		t.Fatal("expected nil command from chatMsg error")
	}
}

func TestUpdateHandlesExecOutput(t *testing.T) {
	m, _ := setupWorkspace(t)

	_, cmd := m.Update(execOutput{output: "hello world"})
	if len(m.terminal.history) != 1 {
		t.Fatalf("expected 1 terminal history entry, got %d", len(m.terminal.history))
	}
	if m.terminal.history[0] != "hello world" {
		t.Fatalf("unexpected terminal output: %q", m.terminal.history[0])
	}
	if cmd != nil {
		t.Fatal("expected nil command from execOutput")
	}
}

func TestViewReturnsNonEmptyAfterResize(t *testing.T) {
	m, _ := setupWorkspace(t)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := m.View()
	if v == "" {
		t.Fatal("expected non-empty View after resize")
	}
}

func TestViewLoadingBeforeResize(t *testing.T) {
	m, _ := setupWorkspace(t)
	v := m.View()
	if v != "Loading..." {
		t.Fatalf("expected 'Loading...', got %q", v)
	}
}

func TestFocusStartsWithFileTree(t *testing.T) {
	m, _ := setupWorkspace(t)
	if m.activePane != paneFileTree {
		t.Fatalf("expected initial focus on file tree, got %d", m.activePane)
	}
}

func TestFocusSwitchingTabFromFileTree(t *testing.T) {
	m, _ := setupWorkspace(t)

	if m.activePane != paneFileTree {
		t.Fatalf("expected initial focus on file tree, got %d", m.activePane)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activePane != paneChat {
		t.Fatalf("expected pane chat after tab, got %d", m.activePane)
	}
}

func TestFocusSwitchingFileTreeToChat(t *testing.T) {
	m, _ := setupWorkspace(t)

	if m.activePane != paneFileTree {
		t.Fatalf("expected initial focus on file tree, got %d", m.activePane)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activePane != paneChat {
		t.Fatalf("expected pane chat after tab from filetree, got %d", m.activePane)
	}
}

func TestTerminalPaneConsumesTabEvent(t *testing.T) {
	m, _ := setupWorkspace(t)
	m.activePane = paneTerminal

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activePane != paneTerminal {
		t.Fatal("expected terminal pane to consume tab event")
	}
}

func TestChatPaneConsumesTabEvent(t *testing.T) {
	m, _ := setupWorkspace(t)
	m.activePane = paneChat

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activePane != paneChat {
		t.Fatal("expected chat pane to consume tab event")
	}
}

func TestArrowKeysNavigateFileTree(t *testing.T) {
	m, _ := setupWorkspace(t)

	initialCursor := m.cursor

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor <= initialCursor {
		t.Fatal("expected cursor to move down")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != initialCursor {
		t.Fatalf("expected cursor to return to %d, got %d", initialCursor, m.cursor)
	}
}

func TestArrowKeysIgnoredInChatPane(t *testing.T) {
	m, _ := setupWorkspace(t)

	m.activePane = paneChat
	initialHistory := len(m.chatHistory)

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if len(m.chatHistory) != initialHistory {
		t.Fatal("expected key event to be ignored in chat pane")
	}
}

func TestCtrlCQuits(t *testing.T) {
	m, _ := setupWorkspace(t)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command from Ctrl+C")
	}
}

func TestQuitKey(t *testing.T) {
	m, _ := setupWorkspace(t)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command from 'q'")
	}
}

func TestChatHistoryAppendedOnSend(t *testing.T) {
	m, _ := setupWorkspace(t)
	m.activePane = paneChat

	m.chatInput = "hello"
	m.handleChatInput(tea.KeyMsg{Type: tea.KeyEnter})

	found := false
	for _, msg := range m.chatHistory {
		if msg.content == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'hello' in chat history after sending")
	}
}

func TestChatInputClearedAfterSend(t *testing.T) {
	m, _ := setupWorkspace(t)
	m.activePane = paneChat

	m.chatInput = "hello"
	m.handleChatInput(tea.KeyMsg{Type: tea.KeyEnter})

	if m.chatInput != "" {
		t.Fatal("expected chat input to be cleared after send")
	}
}

func TestChatBackspace(t *testing.T) {
	m, _ := setupWorkspace(t)
	m.activePane = paneChat

	m.chatInput = "hello"
	m.handleChatInput(tea.KeyMsg{Type: tea.KeyBackspace})

	if m.chatInput != "hell" {
		t.Fatalf("expected 'hell', got %q", m.chatInput)
	}
}

func TestChatTypeCharacter(t *testing.T) {
	m, _ := setupWorkspace(t)
	m.activePane = paneChat

	m.handleChatInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.chatInput != "a" {
		t.Fatalf("expected 'a', got %q", m.chatInput)
	}
}

func TestToggleTerminalShowHide(t *testing.T) {
	m, _ := setupWorkspace(t)

	if m.showTerminal {
		t.Fatal("expected terminal hidden initially")
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if m.showTerminal {
		t.Fatal("expected terminal still hidden after unrelated key")
	}
}

func TestToggleTerminalViaCtrlT(t *testing.T) {
	m, _ := setupWorkspace(t)

	before := m.showTerminal

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	_ = cmd

	if m.showTerminal == before {
		t.Fatal("expected showTerminal to toggle on Ctrl+T")
	}
}

func TestTerminalView(t *testing.T) {
	tp := TerminalPane{
		shown:  true,
		height: 10,
		history: []string{
			"$ echo hello",
			"hello",
		},
		input: "ls",
	}

	v := tp.View(80)
	if v == "" {
		t.Fatal("expected non-empty terminal view")
	}
}

func TestTerminalHistoryTruncation(t *testing.T) {
	tp := TerminalPane{
		shown:  true,
		height: 5,
	}

	for i := 0; i < 20; i++ {
		tp.history = append(tp.history, "line")
	}

	v := tp.View(80)
	if v == "" {
		t.Fatal("expected non-empty terminal view with many lines")
	}
}

func TestRenderSidebar(t *testing.T) {
	m, _ := setupWorkspace(t)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	sidebar := m.renderSidebar(30)
	if sidebar == "" {
		t.Fatal("expected non-empty sidebar")
	}
}

func TestRenderChat(t *testing.T) {
	m, _ := setupWorkspace(t)

	m.chatHistory = append(m.chatHistory, chatMessage{role: "user", content: "hi"})
	m.chatHistory = append(m.chatHistory, chatMessage{role: "assistant", content: "hello"})

	chat := m.renderChat()
	if chat == "" {
		t.Fatal("expected non-empty chat render")
	}
}

func TestRenderStatus(t *testing.T) {
	m, _ := setupWorkspace(t)

	status := m.renderStatus()
	if status == "" {
		t.Fatal("expected non-empty status bar")
	}
}

func TestLoadFiles(t *testing.T) {
	m, _ := setupWorkspace(t)

	if len(m.files) == 0 {
		t.Fatal("expected files to be loaded")
	}

	found := false
	for _, f := range m.files {
		if f.name == "file1.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'file1.txt' to be in files list")
	}
}

func TestCursorBounds(t *testing.T) {
	m, _ := setupWorkspace(t)

	for i := 0; i < 50; i++ {
		m.cursorDown()
	}

	m.cursorUp()
	if m.cursor < 0 {
		t.Fatal("cursor should never go negative")
	}
}

func TestExpandAndCollapseDir(t *testing.T) {
	m, _ := setupWorkspace(t)

	var dirEntry *fileEntry
	for i := range m.files {
		if m.files[i].isDir {
			dirEntry = &m.files[i]
			break
		}
	}
	if dirEntry == nil {
		t.Skip("no directory entry to expand")
	}

	m.expandDir(dirEntry)
	if !m.expanded[dirEntry.path] {
		t.Fatal("expected directory to be expanded")
	}
	if dirEntry.children == nil {
		t.Fatal("expected children to be loaded after expansion")
	}

	m.collapseDir(dirEntry)
	if m.expanded[dirEntry.path] {
		t.Fatal("expected directory to be collapsed")
	}
	if dirEntry.children != nil {
		t.Fatal("expected children to be cleared after collapse")
	}
}

func TestToggleExpand(t *testing.T) {
	m, _ := setupWorkspace(t)

	idx := -1
	for i := range m.files {
		if m.files[i].isDir {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Skip("no directory entry to toggle")
	}

	m.cursor = idx
	m.toggleExpand()

	entry := m.entryAt(m.cursor)
	if entry == nil || !entry.isDir {
		t.Skip("cursor not on directory")
	}
	if !m.expanded[entry.path] {
		t.Fatal("expected directory to be expanded after toggle")
	}
}

func assert(msg string) error {
	return &assertError{msg: msg}
}

type assertError struct{ msg string }

func (e *assertError) Error() string { return e.msg }
