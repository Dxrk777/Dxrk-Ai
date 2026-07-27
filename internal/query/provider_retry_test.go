// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// errProvider simulates an LLM that always fails.
type errProvider struct {
	msg string
}

func (e *errProvider) Generate(_ context.Context, _ []Message, _ []ToolSchema) (Response, error) {
	return Response{}, errors.New(e.msg)
}

// countProvider delegates to a counter function.
type countProvider struct {
	fn func(ctx context.Context, msgs []Message, tools []ToolSchema) (Response, error)
}

func (c *countProvider) Generate(ctx context.Context, msgs []Message, tools []ToolSchema) (Response, error) {
	return c.fn(ctx, msgs, tools)
}

func TestRetryProvider_SuccessFirstTry(t *testing.T) {
	primary := &mockProvider{
		responses: []mockResponse{
			{text: "ok", usage: Usage{InputTokens: 5, OutputTokens: 5}},
		},
	}
	rp := NewRetryProvider(primary)
	resp, err := rp.Generate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "ok" {
		t.Fatalf("Text = %q, want %q", resp.Text, "ok")
	}
}

func TestRetryProvider_RetryThenSuccess(t *testing.T) {
	var attempts atomic.Int32
	primary := &countProvider{
		fn: func(ctx context.Context, msgs []Message, tools []ToolSchema) (Response, error) {
			n := attempts.Add(1)
			if int(n) < 3 {
				return Response{}, errors.New("transient error")
			}
			return Response{Text: "ok", Usage: Usage{InputTokens: 5, OutputTokens: 5}}, nil
		},
	}

	rp := NewRetryProvider(primary, WithMaxRetries(3), WithBaseDelay(time.Millisecond))
	resp, err := rp.Generate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "ok" {
		t.Fatalf("Text = %q, want %q", resp.Text, "ok")
	}
	if n := attempts.Load(); n != 3 {
		t.Fatalf("attempts = %d, want 3", n)
	}
}

func TestRetryProvider_Fallback(t *testing.T) {
	fail := &errProvider{msg: "always fails"}
	fallback := &mockProvider{
		responses: []mockResponse{
			{text: "fallback ok", usage: Usage{InputTokens: 3, OutputTokens: 3}},
		},
	}

	rp := NewRetryProvider(fail,
		WithFallback(fallback),
		WithMaxRetries(2),
		WithBaseDelay(time.Millisecond),
	)

	resp, err := rp.Generate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "fallback ok" {
		t.Fatalf("Text = %q, want %q", resp.Text, "fallback ok")
	}
}

func TestRetryProvider_AllFailNoFallback(t *testing.T) {
	fail := &errProvider{msg: "always fails"}
	rp := NewRetryProvider(fail, WithMaxRetries(2), WithBaseDelay(time.Millisecond))
	_, err := rp.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRetryProvider_ContextCancel(t *testing.T) {
	fail := &errProvider{msg: "fail"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rp := NewRetryProvider(fail, WithMaxRetries(5), WithBaseDelay(time.Hour))
	_, err := rp.Generate(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRetryProvider_ZeroMaxRetries(t *testing.T) {
	var attempts atomic.Int32
	primary := &countProvider{
		fn: func(ctx context.Context, msgs []Message, tools []ToolSchema) (Response, error) {
			attempts.Add(1)
			return Response{}, errors.New("fail")
		},
	}

	rp := NewRetryProvider(primary, WithMaxRetries(0), WithBaseDelay(time.Millisecond))
	_, err := rp.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if n := attempts.Load(); n != 1 {
		t.Fatalf("attempts = %d, want 1 (maxRetries=0 means no retries)", n)
	}
}

func TestRetryProvider_ZeroBaseDelay(t *testing.T) {
	var attempts atomic.Int32
	primary := &countProvider{
		fn: func(ctx context.Context, msgs []Message, tools []ToolSchema) (Response, error) {
			n := attempts.Add(1)
			if n < 2 {
				return Response{}, errors.New("transient")
			}
			return Response{Text: "ok", Usage: Usage{InputTokens: 1, OutputTokens: 1}}, nil
		},
	}

	rp := NewRetryProvider(primary, WithMaxRetries(2), WithBaseDelay(2*time.Microsecond))
	resp, err := rp.Generate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "ok" {
		t.Fatalf("Text = %q, want %q", resp.Text, "ok")
	}
	if n := attempts.Load(); n != 2 {
		t.Fatalf("attempts = %d, want 2", n)
	}
}

func TestRetryProvider_FallbackError(t *testing.T) {
	fail := &errProvider{msg: "primary failure"}
	fallback := &errProvider{msg: "fallback failure"}

	rp := NewRetryProvider(fail,
		WithFallback(fallback),
		WithMaxRetries(1),
		WithBaseDelay(time.Millisecond),
	)

	_, err := rp.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "primary failure") {
		t.Fatalf("error = %q, want it to contain 'primary failure'", errStr)
	}
	if !strings.Contains(errStr, "fallback failure") {
		t.Fatalf("error = %q, want it to contain 'fallback failure'", errStr)
	}
}

func TestRetryProvider_NilFallback(t *testing.T) {
	rp := NewRetryProvider(&errProvider{msg: "fail"},
		WithMaxRetries(1),
		WithBaseDelay(time.Millisecond),
	)
	_, err := rp.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRetryProvider_DefaultValues(t *testing.T) {
	rp := NewRetryProvider(&mockProvider{})
	if rp.maxRetries != 3 {
		t.Fatalf("maxRetries = %d, want 3", rp.maxRetries)
	}
	if rp.baseDelay != time.Second {
		t.Fatalf("baseDelay = %v, want 1s", rp.baseDelay)
	}
	if rp.fallback != nil {
		t.Fatal("fallback should be nil by default")
	}
}

func TestRetryProvider_ExhaustedErrorContainsLastError(t *testing.T) {
	fail := &errProvider{msg: "last error message"}
	rp := NewRetryProvider(fail, WithMaxRetries(1), WithBaseDelay(time.Millisecond))
	_, err := rp.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "last error message") {
		t.Fatalf("error = %q, want to contain 'last error message'", err.Error())
	}
}
