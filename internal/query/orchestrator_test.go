// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

func TestNewOrchestrator_Valid(t *testing.T) {
	reg := tools.New()
	o := NewOrchestrator(reg)
	if o == nil {
		t.Fatal("NewOrchestrator() returned nil")
	}
	if o.registry != reg {
		t.Fatal("NewOrchestrator() did not set registry")
	}
}

func TestNewOrchestrator_WithToolContext(t *testing.T) {
	reg := tools.New()
	tCtx := tools.Context{}
	o := NewOrchestrator(reg, tCtx)
	if o == nil {
		t.Fatal("NewOrchestrator() returned nil")
	}
}

func TestOrchestratorExecute_EmptyBlocks(t *testing.T) {
	reg := tools.New()
	o := NewOrchestrator(reg)

	results, err := o.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute(nil) error = %v", err)
	}
	if results != nil {
		t.Fatalf("Execute(nil) = %v, want nil", results)
	}

	results, err = o.Execute(context.Background(), []ToolUseBlock{})
	if err != nil {
		t.Fatalf("Execute([]) error = %v", err)
	}
	if results != nil {
		t.Fatalf("Execute([]) = %v, want nil", results)
	}
}

func TestOrchestratorExecute_Success(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "echo", false, "hello world")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	o := NewOrchestrator(reg)
	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "echo", Input: map[string]any{}, Index: 0},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Content != "hello world" {
		t.Fatalf("Content = %q, want %q", results[0].Content, "hello world")
	}
	if results[0].IsError {
		t.Fatal("IsError = true, want false")
	}
	if results[0].ToolUseID != "tu_1" {
		t.Fatalf("ToolUseID = %q, want %q", results[0].ToolUseID, "tu_1")
	}
}

func TestOrchestratorExecute_ToolReturnsError(t *testing.T) {
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "failing",
		Description: "always fails",
		Execute: func(_ tools.Context, _ map[string]any) (any, error) {
			return nil, fmt.Errorf("something went wrong")
		},
	})
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	o := NewOrchestrator(reg)
	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "failing", Input: map[string]any{}, Index: 0},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !results[0].IsError {
		t.Fatal("IsError = false, want true")
	}
	if results[0].Content != "error: something went wrong" {
		t.Fatalf("Content = %q, want %q", results[0].Content, "error: something went wrong")
	}
}

func TestOrchestratorExecute_UnknownTool(t *testing.T) {
	reg := tools.New()
	o := NewOrchestrator(reg)

	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "nonexistent", Input: map[string]any{}, Index: 0},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !results[0].IsError {
		t.Fatal("IsError = false, want true")
	}
}

func TestOrchestratorExecute_MultipleConcurrent(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "tool_a", true, "result_a")); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if err := reg.Register(newTestTool(t, "tool_b", true, "result_b")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	o := NewOrchestrator(reg)
	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "tool_a", Input: map[string]any{}, Index: 0},
		{ID: "tu_2", Name: "tool_b", Input: map[string]any{}, Index: 1},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	resultMap := make(map[string]string)
	for _, r := range results {
		resultMap[r.ToolUseID] = r.Content
	}
	if resultMap["tu_1"] != "result_a" {
		t.Fatalf("tu_1 content = %q, want %q", resultMap["tu_1"], "result_a")
	}
	if resultMap["tu_2"] != "result_b" {
		t.Fatalf("tu_2 content = %q, want %q", resultMap["tu_2"], "result_b")
	}
}

func TestOrchestratorExecute_MixedConcurrentAndSerial(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "concurrent_tool", true, "concurrent")); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if err := reg.Register(newTestTool(t, "serial_tool", false, "serial")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	o := NewOrchestrator(reg)
	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "concurrent_tool", Input: map[string]any{}, Index: 0},
		{ID: "tu_2", Name: "concurrent_tool", Input: map[string]any{}, Index: 1},
		{ID: "tu_3", Name: "serial_tool", Input: map[string]any{}, Index: 2},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	resultMap := make(map[string]string)
	for _, r := range results {
		resultMap[r.ToolUseID] = r.Content
	}
	if resultMap["tu_1"] != "concurrent" {
		t.Fatalf("tu_1 content = %q, want %q", resultMap["tu_1"], "concurrent")
	}
	if resultMap["tu_2"] != "concurrent" {
		t.Fatalf("tu_2 content = %q, want %q", resultMap["tu_2"], "concurrent")
	}
	if resultMap["tu_3"] != "serial" {
		t.Fatalf("tu_3 content = %q, want %q", resultMap["tu_3"], "serial")
	}
}

func TestOrchestratorExecute_ToolWithMapResult(t *testing.T) {
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "map_result",
		Description: "returns a map",
		Execute: func(_ tools.Context, _ map[string]any) (any, error) {
			return map[string]any{"key": "value", "number": 42}, nil
		},
	})
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	o := NewOrchestrator(reg)
	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "map_result", Input: map[string]any{}, Index: 0},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].IsError {
		t.Fatal("IsError = true, want false")
	}
	if results[0].Content == "" {
		t.Fatal("Content is empty, expected JSON")
	}
}

func TestOrchestratorExecute_ToolWithBytesResult(t *testing.T) {
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "bytes_result",
		Description: "returns bytes",
		Execute: func(_ tools.Context, _ map[string]any) (any, error) {
			return []byte("bytes result"), nil
		},
	})
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	o := NewOrchestrator(reg)
	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "bytes_result", Input: map[string]any{}, Index: 0},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Content != "bytes result" {
		t.Fatalf("Content = %q, want %q", results[0].Content, "bytes result")
	}
}

func TestOrchestratorExecute_CancelledContext(t *testing.T) {
	reg := tools.New()
	if err := reg.Register(newTestTool(t, "echo", false, "hello")); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	o := NewOrchestrator(reg)
	results, err := o.Execute(ctx, []ToolUseBlock{
		{ID: "tu_1", Name: "echo", Input: map[string]any{}, Index: 0},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
}

func TestOrchestratorExecute_ContextCancelledDuringConcurrentTool(t *testing.T) {
	reg := tools.New()
	var started atomic.Bool
	tool, err := tools.Build(tools.ToolDef{
		Name:             "slow",
		Description:      "slow tool",
		IsConcurrentSafe: func() *bool { b := true; return &b }(),
		Execute: func(ctx tools.Context, _ map[string]any) (any, error) {
			started.Store(true)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	o := NewOrchestrator(reg)

	errCh := make(chan error, 1)
	go func() {
		_, err := o.Execute(ctx, []ToolUseBlock{
			{ID: "tu_1", Name: "slow", Input: map[string]any{}, Index: 0},
		})
		errCh <- err
	}()

	for !started.Load() {
		time.Sleep(time.Millisecond)
	}
	cancel()

	<-errCh
}

func TestOrchestratorExecute_MarshalErrorResult(t *testing.T) {
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "bad_result",
		Description: "returns a value that can't be marshaled",
		Execute: func(_ tools.Context, _ map[string]any) (any, error) {
			return make(chan int), nil
		},
	})
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	o := NewOrchestrator(reg)
	results, err := o.Execute(context.Background(), []ToolUseBlock{
		{ID: "tu_1", Name: "bad_result", Input: map[string]any{}, Index: 0},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !results[0].IsError {
		t.Fatal("IsError = false, want true")
	}
}
