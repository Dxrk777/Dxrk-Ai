// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/tools"
)

func BenchmarkLoopSuccess(b *testing.B) {
	reg := tools.New()
	provider := &mockProvider{
		responses: []mockResponse{
			{text: "Hello!", usage: Usage{InputTokens: 10, OutputTokens: 5}},
		},
	}
	l := New(provider, reg)
	msgs := []Message{
		{Role: RoleUser, Content: "Say hi"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.cursor.Store(0)
		_, _ = l.Run(context.Background(), msgs)
	}
}

func BenchmarkLoopSingleTool(b *testing.B) {
	reg := tools.New()
	if err := reg.Register(newBenchTool(b, "greet", false, "hello back")); err != nil {
		b.Fatalf("Register error = %v", err)
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
	l := New(provider, reg)
	msgs := []Message{
		{Role: RoleUser, Content: "Use the greet tool"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.cursor.Store(0)
		_, _ = l.Run(context.Background(), msgs)
	}
}

func BenchmarkLoopConcurrent(b *testing.B) {
	reg := tools.New()
	if err := reg.Register(newBenchTool(b, "read_a", true, "file a content")); err != nil {
		b.Fatalf("Register error = %v", err)
	}
	if err := reg.Register(newBenchTool(b, "read_b", true, "file b content")); err != nil {
		b.Fatalf("Register error = %v", err)
	}
	provider := &mockProvider{
		responses: []mockResponse{
			{
				text: "Reading files...",
				toolUses: []ToolUseBlock{
					{ID: "tu_1", Name: "read_a", Input: map[string]any{}, Index: 0},
					{ID: "tu_2", Name: "read_b", Input: map[string]any{}, Index: 1},
				},
				usage: Usage{InputTokens: 20, OutputTokens: 10},
			},
			{
				text:  "Both files read",
				usage: Usage{InputTokens: 30, OutputTokens: 5},
			},
		},
	}
	l := New(provider, reg)
	msgs := []Message{
		{Role: RoleUser, Content: "Read both files"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.cursor.Store(0)
		_, _ = l.Run(context.Background(), msgs)
	}
}

func newBenchTool(b *testing.B, name string, concurrentSafe bool, result string) tools.Tool {
	b.Helper()
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
		b.Fatalf("Build(%q) error = %v", name, err)
	}
	return tool
}
