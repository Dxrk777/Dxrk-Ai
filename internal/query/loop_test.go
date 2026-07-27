// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/compress"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

// mockProvider simulates an LLM for testing.
type mockProvider struct {
	responses []mockResponse
	cursor    atomic.Int32
}

type mockResponse struct {
	text     string
	toolUses []ToolUseBlock
	usage    Usage
}

func (m *mockProvider) Generate(_ context.Context, _ []Message, _ []ToolSchema) (Response, error) {
	i := int(m.cursor.Add(1)) - 1
	if i >= len(m.responses) {
		// Repeat last response when exhausted
		r := m.responses[len(m.responses)-1]
		copied := make([]ToolUseBlock, len(r.toolUses))
		copy(copied, r.toolUses)
		return Response{Text: r.text, ToolUses: copied, Usage: r.usage}, nil
	}
	r := m.responses[i]
	copied := make([]ToolUseBlock, len(r.toolUses))
	copy(copied, r.toolUses)
	return Response{Text: r.text, ToolUses: copied, Usage: r.usage}, nil
}

func newTestTool(t *testing.T, name string, concurrentSafe bool, result string) tools.Tool {
	t.Helper()
	cSafe := &concurrentSafe
	readOnly := &concurrentSafe
	tool, err := tools.Build(tools.ToolDef{
		Name:             name,
		Description:      "test tool " + name,
		IsConcurrentSafe: cSafe,
		IsReadOnly:       readOnly,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return result, nil
		},
	})
	if err != nil {
		t.Fatalf("Build(%q) error = %v", name, err)
	}
	return tool
}

func TestNew(t *testing.T) {
	reg := tools.New()
	l := New(&mockProvider{}, reg)
	if l == nil {
		t.Fatal("New() returned nil")
	}
	if l.maxTurns != 25 {
		t.Fatalf("maxTurns = %d, want 25", l.maxTurns)
	}
}

func TestRun_SuccessNoTools(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{text: "Hello, world!", usage: Usage{InputTokens: 10, OutputTokens: 5}},
		},
	}
	l := New(provider, reg)

	result, err := l.Run(context.Background(), []Message{
		{Role: RoleSystem, Content: "You are a helpful assistant."},
		{Role: RoleUser, Content: "Say hi"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopSuccess {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
	if !strings.Contains(result.FinalText, "Hello") {
		t.Fatalf("FinalText = %q, want containing 'Hello'", result.FinalText)
	}
	if result.ToolCalls != 0 {
		t.Fatalf("ToolCalls = %d, want 0", result.ToolCalls)
	}
	if result.Turns != 0 {
		t.Fatalf("Turns = %d, want 0", result.Turns)
	}
}

func TestRun_SingleToolCall(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "greet", false, "hello back")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Let me use a tool.",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "greet", Input: map[string]any{}, Index: 0},
				},
				usage: Usage{InputTokens: 20, OutputTokens: 10},
			},
			{
				text:  "Done! The result was hello back",
				usage: Usage{InputTokens: 30, OutputTokens: 5},
			},
		},
	}

	l := New(provider, reg)
	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Use the greet tool"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopSuccess {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
	if result.ToolCalls != 1 {
		t.Fatalf("ToolCalls = %d, want 1", result.ToolCalls)
	}
	if result.Turns != 1 {
		t.Fatalf("Turns = %d, want 1", result.Turns)
	}
}

func TestRun_MaxTurns(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Using tool...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "sing", Input: map[string]any{}, Index: 0},
				},
			},
		},
	}

	// Register so it doesn't fail, but keep looping
	if err := reg.Register(newTestTool(t, "sing", false, "la la la")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	l := New(provider, reg, WithMaxTurns(3))
	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Sing a song"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopMaxTurns {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopMaxTurns)
	}
	if result.Turns != 3 {
		t.Fatalf("Turns = %d, want 3", result.Turns)
	}
}

func TestRun_Interrupted(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Using tool...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "slow", Input: map[string]any{}, Index: 0},
				},
			},
		},
	}
	if err := reg.Register(newTestTool(t, "slow", false, "done")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	var interruptCalled bool
	l := New(provider, reg, WithInterrupt(func() bool {
		if !interruptCalled {
			interruptCalled = true
			return false
		}
		return true
	}))

	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Go slow"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopInterrupted {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopInterrupted)
	}
}

func TestRun_TurnCallback(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "calc", false, "42")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Calculating...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "calc", Input: map[string]any{}, Index: 0},
				},
			},
			{
				text: "The answer is 42",
			},
		},
	}

	var turn, toolCount int
	l := New(provider, reg, WithTurnCallback(func(t, msgs, tc int) {
		turn = t
		_ = msgs
		toolCount = tc
	}))

	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "What is the answer?"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Turns != 1 {
		t.Fatalf("Turns = %d, want 1", result.Turns)
	}
	if turn != 1 {
		t.Fatalf("callback turn = %d, want 1", turn)
	}
	if toolCount != 1 {
		t.Fatalf("callback toolCount = %d, want 1", toolCount)
	}
}

func TestRun_UnknownTool(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Using unknown tool...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "nonexistent", Input: map[string]any{}, Index: 0},
				},
			},
			{
				text: "Fixed it",
			},
		},
	}
	l := New(provider, reg)
	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Do something"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopSuccess {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
	// Unknown tool should be handled gracefully (error result)
}

func TestRun_ConcurrentTools(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "read_a", true, "file a content")); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if err := reg.Register(newTestTool(t, "read_b", true, "file b content")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Reading files...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "read_a", Input: map[string]any{}, Index: 0},
					{ID: "tu_2", Name: "read_b", Input: map[string]any{}, Index: 1},
				},
			},
			{
				text: "Both files read",
			},
		},
	}
	l := New(provider, reg)
	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Read both files"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopSuccess {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
	if result.ToolCalls != 2 {
		t.Fatalf("ToolCalls = %d, want 2", result.ToolCalls)
	}
}

func TestRun_CancelViaContext(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Working...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "task", Input: map[string]any{}, Index: 0},
				},
			},
		},
	}
	if err := reg.Register(newTestTool(t, "task", false, "done")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	l := New(provider, reg)
	result, err := l.Run(ctx, []Message{
		{Role: RoleUser, Content: "Do it"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopInterrupted {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopInterrupted)
	}
}

// --- With* option tests ---

func TestWithMaxTurns(t *testing.T) {
	l := &Loop{}
	WithMaxTurns(10)(l)
	if l.maxTurns != 10 {
		t.Fatalf("maxTurns = %d, want 10", l.maxTurns)
	}
}

func TestWithCompressor(t *testing.T) {
	l := &Loop{}
	c := compress.New(compress.WithMaxTokens(100))
	WithCompressor(c)(l)
	if l.compressor != c {
		t.Fatal("compressor not set")
	}
}

func TestWithBudget(t *testing.T) {
	l := &Loop{}
	b := compress.NewBudget(1000)
	WithBudget(b)(l)
	if l.budget != b {
		t.Fatal("budget not set")
	}
}

func TestWithInterrupt(t *testing.T) {
	l := &Loop{}
	fn := func() bool { return true }
	WithInterrupt(fn)(l)
	if l.interrupt == nil {
		t.Fatal("interrupt is nil")
	}
	if !l.interrupt() {
		t.Fatal("interrupt returned false, want true")
	}
}

func TestWithTurnCallback(t *testing.T) {
	l := &Loop{}
	called := false
	fn := func(turn, msgs, toolCount int) { called = true }
	WithTurnCallback(fn)(l)
	if l.onTurn == nil {
		t.Fatal("onTurn is nil")
	}
	l.onTurn(1, 2, 3)
	if !called {
		t.Fatal("callback was not called")
	}
}

func TestWithChecker(t *testing.T) {
	l := &Loop{}
	WithChecker(nil, nil)(l)
	if l.checker != nil {
		t.Fatal("checker should be nil")
	}
	if l.audit != nil {
		t.Fatal("audit should be nil")
	}
}

func TestWithPersistence(t *testing.T) {
	l := &Loop{}
	m := &mockPersistence{}
	WithPersistence(m)(l)
	if l.persistence != m {
		t.Fatal("persistence not set")
	}
}

func TestWithTracer(t *testing.T) {
	l := &Loop{}
	WithTracer(nil)(l)
	if l.tp != nil {
		t.Fatal("tracer should be nil")
	}
}

func TestNewWithAllOptions(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{}
	l := New(provider, reg,
		WithMaxTurns(5),
		WithCompressor(compress.New(compress.WithMaxTokens(100))),
		WithBudget(compress.NewBudget(500)),
		WithInterrupt(func() bool { return true }),
		WithTurnCallback(func(int, int, int) {}),
		WithChecker(nil, nil),
		WithPersistence(&mockPersistence{}),
		WithTracer(nil),
	)
	if l.maxTurns != 5 {
		t.Fatalf("maxTurns = %d, want 5", l.maxTurns)
	}
	if l.compressor == nil {
		t.Fatal("compressor not set")
	}
	if l.budget == nil {
		t.Fatal("budget not set")
	}
	if l.interrupt == nil {
		t.Fatal("interrupt not set")
	}
	if l.onTurn == nil {
		t.Fatal("onTurn not set")
	}
	if l.persistence == nil {
		t.Fatal("persistence not set")
	}
}

// --- buildToolSchemas tests ---

func TestBuildToolSchemas_EmptyRegistry(t *testing.T) {
	reg := tools.New()
	l := New(&mockProvider{}, reg)
	schemas := l.buildToolSchemas()
	if len(schemas) != 0 {
		t.Fatalf("len(schemas) = %d, want 0", len(schemas))
	}
}

func TestBuildToolSchemas_WithTools(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "tool_a", false, "result_a")); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if err := reg.Register(newTestTool(t, "tool_b", false, "result_b")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	l := New(&mockProvider{}, reg)
	schemas := l.buildToolSchemas()
	if len(schemas) != 2 {
		t.Fatalf("len(schemas) = %d, want 2", len(schemas))
	}

	names := make([]string, len(schemas))
	for i, s := range schemas {
		names[i] = s.Name
	}
	if names[0] != "tool_a" {
		t.Fatalf("names[0] = %q, want %q", names[0], "tool_a")
	}
	if names[1] != "tool_b" {
		t.Fatalf("names[1] = %q, want %q", names[1], "tool_b")
	}
}

func TestBuildToolSchemas_InputSchemaPassthrough(t *testing.T) {
	reg := tools.New()
	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	tool, err := tools.Build(tools.ToolDef{
		Name:        "complex",
		Description: "complex tool",
		InputSchema: inputSchema,
		IsEnabled:   tools.DefaultEnabled(),
		Execute: func(_ tools.Context, _ map[string]any) (any, error) {
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	l := New(&mockProvider{}, reg)
	schemas := l.buildToolSchemas()
	if len(schemas) != 1 {
		t.Fatalf("len(schemas) = %d, want 1", len(schemas))
	}
	if schemas[0].Name != "complex" {
		t.Fatalf("Name = %q, want %q", schemas[0].Name, "complex")
	}
	if schemas[0].Description != "complex tool" {
		t.Fatalf("Description = %q, want %q", schemas[0].Description, "complex tool")
	}
	if schemas[0].InputSchema == nil {
		t.Fatal("InputSchema is nil")
	}
}

// --- persistTurn tests ---

func TestPersistTurn_NilPersistence(t *testing.T) {
	l := &Loop{}
	l.persistTurn([]Message{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi!"},
	})
	// Should not panic
}

func TestPersistTurn_SavesUserAndAssistant(t *testing.T) {
	m := &mockPersistence{}
	l := &Loop{persistence: m}

	l.persistTurn([]Message{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there!"},
	})

	m.mu.Lock()
	if len(m.turns) != 1 {
		m.mu.Unlock()
		t.Fatalf("turns = %d, want 1", len(m.turns))
	}
	if m.turns[0].userMsg != "Hello" {
		m.mu.Unlock()
		t.Fatalf("userMsg = %q, want %q", m.turns[0].userMsg, "Hello")
	}
	if m.turns[0].assistantMsg != "Hi there!" {
		m.mu.Unlock()
		t.Fatalf("assistantMsg = %q, want %q", m.turns[0].assistantMsg, "Hi there!")
	}
	m.mu.Unlock()
}

func TestPersistTurn_UsesLastMessages(t *testing.T) {
	m := &mockPersistence{}
	l := &Loop{persistence: m}

	l.persistTurn([]Message{
		{Role: RoleUser, Content: "Old user"},
		{Role: RoleAssistant, Content: "Old assistant"},
		{Role: RoleTool, ToolCallID: "tc_1", Content: "tool result"},
		{Role: RoleUser, Content: "New user"},
		{Role: RoleAssistant, Content: "New assistant"},
	})

	m.mu.Lock()
	if len(m.turns) != 1 {
		m.mu.Unlock()
		t.Fatalf("turns = %d, want 1", len(m.turns))
	}
	if m.turns[0].userMsg != "New user" {
		m.mu.Unlock()
		t.Fatalf("userMsg = %q, want %q", m.turns[0].userMsg, "New user")
	}
	if m.turns[0].assistantMsg != "New assistant" {
		m.mu.Unlock()
		t.Fatalf("assistantMsg = %q, want %q", m.turns[0].assistantMsg, "New assistant")
	}
	m.mu.Unlock()
}

func TestPersistTurn_NoUser(t *testing.T) {
	m := &mockPersistence{}
	l := &Loop{persistence: m}

	l.persistTurn([]Message{
		{Role: RoleAssistant, Content: "Hi!"},
	})

	m.mu.Lock()
	if len(m.turns) != 0 {
		m.mu.Unlock()
		t.Fatalf("turns = %d, want 0 (no user message)", len(m.turns))
	}
	m.mu.Unlock()
}

func TestPersistTurn_NoAssistant(t *testing.T) {
	m := &mockPersistence{}
	l := &Loop{persistence: m}

	l.persistTurn([]Message{
		{Role: RoleUser, Content: "Hello"},
	})

	m.mu.Lock()
	if len(m.turns) != 0 {
		m.mu.Unlock()
		t.Fatalf("turns = %d, want 0 (no assistant message)", len(m.turns))
	}
	m.mu.Unlock()
}

func TestPersistTurn_EmptyContent(t *testing.T) {
	m := &mockPersistence{}
	l := &Loop{persistence: m}

	l.persistTurn([]Message{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: ""},
	})

	m.mu.Lock()
	if len(m.turns) != 0 {
		m.mu.Unlock()
		t.Fatalf("turns = %d, want 0 (assistant has empty content)", len(m.turns))
	}
	m.mu.Unlock()
}

func TestPersistTurn_EmptyUserContent(t *testing.T) {
	m := &mockPersistence{}
	l := &Loop{persistence: m}

	l.persistTurn([]Message{
		{Role: RoleUser, Content: ""},
		{Role: RoleAssistant, Content: "Hi"},
	})

	m.mu.Lock()
	if len(m.turns) != 0 {
		m.mu.Unlock()
		t.Fatalf("turns = %d, want 0 (user has empty content)", len(m.turns))
	}
	m.mu.Unlock()
}

func TestPersistTurn_ThroughRun(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "greet", false, "hello back")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Let me use a tool.",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "greet", Input: map[string]any{}, Index: 0},
				},
				usage: Usage{InputTokens: 20, OutputTokens: 10},
			},
			{
				text:  "Done!",
				usage: Usage{InputTokens: 30, OutputTokens: 5},
			},
		},
	}

	m := &mockPersistence{}
	l := New(provider, reg, WithPersistence(m))

	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Use the greet tool"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopSuccess {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}

	m.mu.Lock()
	if len(m.turns) != 1 {
		m.mu.Unlock()
		t.Fatalf("turns = %d, want 1", len(m.turns))
	}
	if m.turns[0].userMsg != "Use the greet tool" {
		m.mu.Unlock()
		t.Fatalf("userMsg = %q, want %q", m.turns[0].userMsg, "Use the greet tool")
	}
	if m.turns[0].assistantMsg != "Let me use a tool." {
		m.mu.Unlock()
		t.Fatalf("assistantMsg = %q, want %q", m.turns[0].assistantMsg, "Let me use a tool.")
	}
	m.mu.Unlock()
}

// --- compression tests ---

func TestCompressMessages_ReducesContent(t *testing.T) {
	c := compress.New(compress.WithMaxTokens(1), compress.WithCompressionPct(99))
	l := &Loop{compressor: c}

	msgs := []Message{
		{Role: RoleUser, Content: strings.Repeat("x", 1000)},
		{Role: RoleAssistant, Content: strings.Repeat("y", 1000)},
	}
	result := l.compressMessages(msgs)
	if len(result) == 0 {
		t.Fatal("compressMessages returned empty")
	}
	if len(result) >= len(msgs) {
		t.Fatalf("compressed len(%d) >= original len(%d), expected fewer messages", len(result), len(msgs))
	}
}

func TestCompressMessages_NoCompressorPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic with nil compressor")
		}
	}()
	l := &Loop{}
	l.compressMessages([]Message{{Role: RoleUser, Content: "Hello"}})
}

func TestRun_WithCompressionEnabled(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "loopback", false, "result")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Using tool...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "loopback", Input: map[string]any{}, Index: 0},
				},
				usage: Usage{InputTokens: 50, OutputTokens: 50},
			},
			{
				text:  "Final answer",
				usage: Usage{InputTokens: 10, OutputTokens: 5},
			},
		},
	}

	// Set up a tight budget so compression triggers after first turn
	budget := compress.NewBudget(100)
	budget.Add(81)

	c := compress.New(compress.WithMaxTokens(1))
	l := New(provider, reg, WithBudget(budget), WithCompressor(c), WithMaxTurns(5))

	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: strings.Repeat("long message ", 50)},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopSuccess {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
}

func TestRun_ResultFields(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{text: "Final text", usage: Usage{InputTokens: 10, OutputTokens: 5}},
		},
	}
	l := New(provider, reg)

	result, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Say something"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.FinalText != "Final text" {
		t.Fatalf("FinalText = %q, want %q", result.FinalText, "Final text")
	}
	if result.Duration <= 0 {
		t.Fatal("Duration should be positive")
	}
	if result.Turns != 0 {
		t.Fatalf("Turns = %d, want 0", result.Turns)
	}
	if result.ToolCalls != 0 {
		t.Fatalf("ToolCalls = %d, want 0", result.ToolCalls)
	}
}

func TestRun_MultipleMessages(t *testing.T) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{text: "Hello!", usage: Usage{InputTokens: 5, OutputTokens: 3}},
		},
	}
	l := New(provider, reg)

	result, err := l.Run(context.Background(), []Message{
		{Role: RoleSystem, Content: "Be helpful."},
		{Role: RoleUser, Content: "Message 1"},
		{Role: RoleAssistant, Content: "Response 1"},
		{Role: RoleUser, Content: "Message 2"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StopReason != StopSuccess {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
	if len(result.Messages) < 5 {
		t.Fatalf("len(Messages) = %d, want >= 5", len(result.Messages))
	}
}

func TestRun_ProviderError(t *testing.T) {
	reg := tools.New()
	provider := &errProvider{msg: "provider failure"}
	l := New(provider, reg)

	_, err := l.Run(context.Background(), []Message{
		{Role: RoleUser, Content: "Do something"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCopyMessages(t *testing.T) {
	now := time.Now()
	original := []Message{
		{Role: RoleUser, Content: "hello", CreatedAt: now},
		{Role: RoleAssistant, Content: "hi", CreatedAt: now},
	}
	copied := copyMessages(original)
	if len(copied) != len(original) {
		t.Fatalf("len(copied) = %d, want %d", len(copied), len(original))
	}
	if copied[0].Content != original[0].Content {
		t.Fatalf("Content = %q, want %q", copied[0].Content, original[0].Content)
	}
	// Verify it's a deep copy
	copied[0].Content = "modified"
	if original[0].Content != "hello" {
		t.Fatal("copyMessages did not create a deep copy")
	}
}
