// SPDX-License-Identifier: MIT
package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/query"
	"github.com/Dxrk777/Dxrk-Ai/internal/router"
)

type benchProvider struct {
	delay time.Duration
}

func (b *benchProvider) Generate(ctx context.Context, msgs []query.Message, tools []query.ToolSchema) (query.Response, error) {
	if b.delay > 0 {
		time.Sleep(b.delay)
	}
	return query.Response{
		Text: "```go\npackage main\n\nfunc main() {}\n```",
		Usage: query.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}, nil
}

func BenchmarkPipeline_Execute_1Iteration(b *testing.B) {
	r := router.NewRouter([]router.ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &benchProvider{}},
	})
	p := NewPipeline(PipelineConfig{
		Router:        r,
		MaxIterations: 1,
		Logger:        func(string, ...any) {},
		CodePrompt:    "You write code.",
		TestPrompt:    "You write tests.",
		ReviewPrompt:  "You always say PASS with no issues.",
		DocsPrompt:    "You write docs.",
		Timeouts:      5 * time.Second,
	})
	task := PipelineTask{
		ID:          "bench-task",
		Description: "Write a Fibonacci function in Go",
		Language:    "go",
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := p.Execute(ctx, task)
		if result.Iterations != 1 {
			b.Fatalf("expected 1 iteration, got %d", result.Iterations)
		}
	}
}

func BenchmarkPipeline_Execute_5Iterations(b *testing.B) {
	r := router.NewRouter([]router.ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &benchProvider{}},
	})
	p := NewPipeline(PipelineConfig{
		Router:        r,
		MaxIterations: 5,
		Logger:        func(string, ...any) {},
		CodePrompt:    "You write code.",
		TestPrompt:    "You write tests.",
		ReviewPrompt:  "You write code reviews with many issues to trigger retries. List at least 5 bugs.",
		DocsPrompt:    "You write docs.",
		Timeouts:      5 * time.Second,
	})
	task := PipelineTask{
		ID:          "bench-task",
		Description: "Write a Fibonacci function in Go",
		Language:    "go",
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := p.Execute(ctx, task)
		if result.Iterations != 5 {
			b.Fatalf("expected 5 iterations, got %d", result.Iterations)
		}
	}
}

func BenchmarkPipeline_llmGenerate(b *testing.B) {
	r := router.NewRouter([]router.ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &benchProvider{}},
	})
	p := NewPipeline(PipelineConfig{
		Router:        r,
		MaxIterations: 1,
		Logger:        func(string, ...any) {},
		CodePrompt:    "You are a coder.",
		TestPrompt:    "You are a tester.",
		ReviewPrompt:  "You are a reviewer.",
		DocsPrompt:    "You are a writer.",
		Timeouts:      5 * time.Second,
	})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.llmGenerate(ctx, RoleCoder, fmt.Sprintf("task %d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildPrompts(b *testing.B) {
	task := PipelineTask{ID: "t1", Description: "Write a Fibonacci function", Language: "go"}
	code := "package main\n\nfunc main() {}\n"
	tests := "package main\n\nfunc TestMain(t *testing.T) {}\n"
	review := "LGTM"
	b.ResetTimer()
	b.Run("CodePrompt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buildCodePrompt(task, code, tests, review)
		}
	})
	b.Run("TestPrompt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buildTestPrompt(task, code)
		}
	})
	b.Run("ReviewPrompt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buildReviewPrompt(task, code, tests)
		}
	})
	b.Run("DocsPrompt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buildDocsPrompt(task, code, tests)
		}
	})
}

func BenchmarkNewPipeline(b *testing.B) {
	r := router.NewRouter([]router.ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &benchProvider{}},
	})
	cfg := DefaultPipelineConfig(r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := NewPipeline(cfg)
		p.Stop()
	}
}
