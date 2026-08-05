// SPDX-License-Identifier: MIT
// Package query implements the agent loop: messages → LLM → tools → repeat.
//
// Ported from Claude Code's query loop (see references/claude-code-source/query.ts):
//   - while(true) { build msgs → call API → execute tools → repeat }
//   - Provider abstraction for LLM backends
//   - ToolOrchestrator for concurrent/safe serial execution
package query

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/components/permissions"
	"github.com/Dxrk777/Dxrk-Ai/internal/compress"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
	"github.com/Dxrk777/Dxrk-Ai/internal/trace"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// StopReason indicates why the loop stopped.
type StopReason string

const (
	StopSuccess     StopReason = strconst.StrSuccess
	StopMaxTurns    StopReason = "max_turns"
	StopError       StopReason = strconst.StrError
	StopInterrupted StopReason = "interrupted"
	StopToolFailure StopReason = "tool_failure"
)

// Role identifies the message sender.
type Role string

const (
	RoleSystem    Role = strconst.StrSystem
	RoleUser      Role = "user"
	RoleAssistant Role = strconst.StrAssistant
	RoleTool      Role = "tool"
)

// Message represents a single message in the conversation.
type Message struct {
	Role       Role      `json:"role"`
	Content    string    `json:"content"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	ToolName   string    `json:"tool_name,omitempty"`
	ToolUseID  string    `json:"tool_use_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToolUseBlock represents a tool call requested by the LLM.
type ToolUseBlock struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
	Index int            `json:"index"`
}

// ToolResultBlock represents the result of a tool execution.
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

// Result holds the final outcome of a query loop run.
type Result struct {
	Messages   []Message     `json:"messages"`
	FinalText  string        `json:"final_text"`
	StopReason StopReason    `json:"stop_reason"`
	Duration   time.Duration `json:"duration"`
	ToolCalls  int           `json:"tool_calls"`
	Turns      int           `json:"turns"`
}

// Loop is the main agent orchestrator.
type Loop struct {
	provider     Provider
	toolRegistry *tools.Registry
	compressor   *compress.Compressor
	budget       *compress.Budget
	maxTurns     int
	checker      *permissions.Checker
	audit        *permissions.Audit
	tp           trace.Exporter
	persistence  Persistence

	interrupt func() bool
	onTurn    func(turn int, msgs int, toolCount int)
}

// Option configures the Loop.
type Option func(*Loop)

// WithMaxTurns sets the maximum number of turns (default 25).
func WithMaxTurns(n int) Option { return func(l *Loop) { l.maxTurns = n } }

// WithCompressor sets the context compressor.
func WithCompressor(c *compress.Compressor) Option { return func(l *Loop) { l.compressor = c } }

// WithBudget sets the token budget tracker.
func WithBudget(b *compress.Budget) Option { return func(l *Loop) { l.budget = b } }

// WithInterrupt sets a function that returns true when the loop should stop.
func WithInterrupt(fn func() bool) Option { return func(l *Loop) { l.interrupt = fn } }

// WithTurnCallback sets a callback called after each turn.
func WithTurnCallback(fn func(turn int, msgs int, toolCount int)) Option {
	return func(l *Loop) { l.onTurn = fn }
}

// WithChecker sets the permission checker and audit log.
func WithChecker(c *permissions.Checker, a *permissions.Audit) Option {
	return func(l *Loop) {
		l.checker = c
		l.audit = a
	}
}

// WithPersistence sets the cross-session persistence backend.
func WithPersistence(p Persistence) Option {
	return func(l *Loop) { l.persistence = p }
}

// WithTracer enables OpenTelemetry tracing.
func WithTracer(tp trace.Exporter) Option {
	return func(l *Loop) { l.tp = tp }
}

// New creates a query Loop.
func New(provider Provider, toolRegistry *tools.Registry, opts ...Option) *Loop {
	l := &Loop{
		provider:     provider,
		toolRegistry: toolRegistry,
		maxTurns:     25,
		interrupt:    func() bool { return false },
		onTurn:       func(int, int, int) {},
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Run executes the agent loop starting from the given messages.
// The loop continues until the LLM returns a text-only response or
// the max turns/interrupt is reached.
func (l *Loop) Run(ctx context.Context, messages []Message) (Result, error) {
	start := time.Now()
	current := copyMessages(messages)
	toolCalls := 0
	turn := 0

	for {
		select {
		case <-ctx.Done():
			return l.result(current, StopInterrupted, start, toolCalls, turn), nil
		default:
		}

		if l.interrupt() {
			return l.result(current, StopInterrupted, start, toolCalls, turn), nil
		}

		if turn >= l.maxTurns {
			return l.result(current, StopMaxTurns, start, toolCalls, turn), nil
		}

		// Apply compression if over budget
		if l.budget != nil && l.budget.NeedsCompression() {
			current = l.compressMessages(current)
		}

		turnCtx, span := l.startSpan(ctx, "turn",
			attribute.Int("turn", turn),
			attribute.Int("messages", len(current)),
		)

		// Build tool schemas for the provider
		toolSchemas := l.buildToolSchemas()

		// Call the LLM
		resp, err := l.provider.Generate(turnCtx, current, toolSchemas)
		if err != nil {
			span.End()
			return Result{}, fmt.Errorf("provider generate: %w", err)
		}

		// Track cost
		if l.budget != nil {
			l.budget.Add(resp.Usage.InputTokens + resp.Usage.OutputTokens)
		}

		// Add assistant message to history
		current = append(current, Message{
			Role:      RoleAssistant,
			Content:   resp.Text,
			CreatedAt: time.Now(),
		})

		// If no tool uses, we're done
		if len(resp.ToolUses) == 0 {
			span.End()
			return l.result(current, StopSuccess, start, toolCalls, turn), nil
		}

		// Execute tools
		toolCtx, toolSpan := l.startSpan(turnCtx, "execute_tools",
			attribute.Int(strconst.StrCount, len(resp.ToolUses)),
		)
		results, err := l.executeTools(toolCtx, resp.ToolUses)
		toolSpan.End()
		if err != nil {
			span.End()
			return l.result(current, StopToolFailure, start, toolCalls, turn), err
		}

		toolCalls += len(results)
		turn++

		// Add tool results to history
		for _, r := range results {
			current = append(current, Message{
				Role:       RoleTool,
				ToolCallID: r.ToolUseID,
				ToolName:   r.Name,
				Content:    r.Content,
				CreatedAt:  time.Now(),
			})
		}

		// Persist turn to DxrkMemory
		l.persistTurn(current)

		l.onTurn(turn, len(current), toolCalls)
		span.End()
	}
}

func (l *Loop) result(msgs []Message, reason StopReason, start time.Time, toolCalls, turns int) Result {
	finalText := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleAssistant && msgs[i].Content != "" {
			finalText = msgs[i].Content
			break
		}
	}
	return Result{
		Messages:   msgs,
		FinalText:  finalText,
		StopReason: reason,
		Duration:   time.Since(start),
		ToolCalls:  toolCalls,
		Turns:      turns,
	}
}

func (l *Loop) compressMessages(msgs []Message) []Message {
	contents := make([]compress.Content, len(msgs))
	for i, m := range msgs {
		contents[i] = compress.Content{
			ID:   fmt.Sprintf("msg-%s-%d", m.Role, i),
			Role: string(m.Role),
			Text: m.Content,
			Size: len(m.Content),
		}
	}
	compressed, _ := l.compressor.Compress(contents)
	result := make([]Message, len(compressed))
	for i, c := range compressed {
		result[i] = Message{
			Role:      Role(c.Role),
			Content:   c.Text,
			CreatedAt: time.Now(),
		}
	}
	return result
}

func (l *Loop) buildToolSchemas() []ToolSchema {
	enabled := l.toolRegistry.ListEnabled()
	schemas := make([]ToolSchema, 0, len(enabled))
	for _, t := range enabled {
		schemas = append(schemas, ToolSchema{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return schemas
}

func (l *Loop) executeTools(ctx context.Context, toolUses []ToolUseBlock) ([]ToolResultBlock, error) {
	tCtx := l.newToolContext(ctx)
	orc := NewOrchestrator(l.toolRegistry, tCtx)
	return orc.Execute(ctx, toolUses)
}

func (l *Loop) newToolContext(ctx context.Context) tools.Context {
	tCtx := tools.Context{Context: ctx}
	if l.checker != nil {
		checker := l.checker
		tCtx.PermissionChecker = permissionCheckerFunc(func(action, target string) (bool, string) {
			r := checker.Check(permissions.Action(action), target)
			return r.Allowed, r.Reason
		})
	}
	if l.audit != nil {
		audit := l.audit
		tCtx.PermissionAudit = auditFunc(func(action, target string, allowed bool, reason string) {
			audit.Record("", permissions.Action(action), target, permissions.Result{Allowed: allowed, Reason: reason})
		})
	}
	return tCtx
}

type permissionCheckerFunc func(action, target string) (bool, string)

func (f permissionCheckerFunc) Check(action, target string) (bool, string) {
	return f(action, target)
}

type auditFunc func(action, target string, allowed bool, reason string)

func (f auditFunc) Record(_ string, target string, allowed bool, reason string) {
	f("tool", target, allowed, reason)
}

func (l *Loop) persistTurn(msgs []Message) {
	if l.persistence == nil {
		return
	}

	var lastUser, lastAssistant string
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if lastAssistant == "" && m.Role == RoleAssistant && m.Content != "" {
			lastAssistant = m.Content
		}
		if lastUser == "" && m.Role == RoleUser && m.Content != "" {
			lastUser = m.Content
		}
		if lastUser != "" && lastAssistant != "" {
			break
		}
	}

	if lastUser == "" || lastAssistant == "" {
		return
	}

	if err := l.persistence.SaveTurn("", lastUser, lastAssistant); err != nil {
		log.Printf("[query] failed to save turn: %v", err)
	}
}

func (l *Loop) startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	if l.tp == nil {
		return ctx, oteltrace.SpanFromContext(ctx)
	}
	return trace.StartSpan(ctx, name, trace.WithAttributes(attrs...))
}

func copyMessages(msgs []Message) []Message {
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out
}
