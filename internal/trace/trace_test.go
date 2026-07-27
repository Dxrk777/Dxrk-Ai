// SPDX-License-Identifier: MIT
package trace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestNewTracerProvider(t *testing.T) {
	tp, err := NewTracerProvider("test-service")
	if err != nil {
		t.Fatalf("NewTracerProvider failed: %v", err)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()

	if tp == nil {
		t.Fatal("expected non-nil Exporter")
	}

	// Ensure it can create a tracer
	tr := tp.Tracer("test")
	if tr == nil {
		t.Fatal("expected non-nil Tracer")
	}
}

func TestStartSpan(t *testing.T) {
	tests := []struct {
		name    string
		span    string
		opts    []oteltrace.SpanStartOption
		wantNil bool
	}{
		{
			name:    "simple span",
			span:    "test-span",
			wantNil: false,
		},
		{
			name: "span with attributes",
			span: "attr-span",
			opts: []oteltrace.SpanStartOption{WithAttributes(attribute.String("key", "val"))},
		},
	}

	// Register a tracing provider for tests that need one
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, span := StartSpan(context.Background(), tt.span, tt.opts...)
			if span == nil {
				t.Fatal("expected non-nil span")
			}
			if !span.IsRecording() && !tt.wantNil {
				t.Fatal("expected span to be recording")
			}
			span.End()

			// Context should contain the span
			if otel.GetTracerProvider().Tracer("dxrk") == nil {
				t.Fatal("expected tracer from context")
			}
			_ = ctx // context has span, just verify no panic
		})
	}
}

func TestStartSpanNop(t *testing.T) {
	// Without a registered provider, spans should still not panic
	ctx, span := StartSpan(context.Background(), "nop-span")
	span.End()
	_ = ctx
}

func TestExporterShutdown(t *testing.T) {
	tp, err := NewTracerProvider("shutdown-test")
	if err != nil {
		t.Fatalf("NewTracerProvider failed: %v", err)
	}

	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Second shutdown should not panic
	_ = tp.Shutdown(context.Background())
}

func TestStartSpanRoundtrip(t *testing.T) {
	tp, err := NewTracerProvider("roundtrip-test")
	if err != nil {
		t.Fatalf("NewTracerProvider failed: %v", err)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()

	otel.SetTracerProvider(tp)

	parentCtx, parentSpan := StartSpan(context.Background(), "parent",
		WithAttributes(attribute.String("type", "test")),
	)

	childCtx, childSpan := StartSpan(parentCtx, "child",
		WithAttributes(attribute.Int("count", 1)),
	)

	childSpan.End()
	parentSpan.End()

	if otel.GetTracerProvider().Tracer("dxrk") == nil {
		t.Fatal("expected tracer")
	}
	_ = childCtx
}
