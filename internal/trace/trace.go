// SPDX-License-Identifier: MIT
// Package trace provides OpenTelemetry tracing for Dxrk.
package trace

import (
	"context"
	"os"

	"github.com/Dxrk777/Dxrk/internal/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Exporter formats and outputs trace spans.
type Exporter interface {
	trace.TracerProvider
	Shutdown(ctx context.Context) error
}

// TracerProvider wraps OTel SDK with stdout export.
// Exists to avoid pulling OTel SDK init into every consumer.
func NewTracerProvider(serviceName string) (Exporter, error) {
	exp, err := stdouttrace.New(
		stdouttrace.WithWriter(os.Stderr),
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, err
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version.Version),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	return &provider{tp, exp}, nil
}

type provider struct {
	*sdktrace.TracerProvider
	ex sdktrace.SpanExporter
}

func (p *provider) Shutdown(ctx context.Context) error {
	if err := p.TracerProvider.Shutdown(ctx); err != nil {
		return err
	}
	return p.ex.Shutdown(ctx)
}

// StartSpan is a convenience wrapper: creates a named span from the global tracer.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer("dxrk").Start(ctx, name, opts...)
}

// WithAttributes attaches key-value pairs to the current span.
func WithAttributes(attrs ...attribute.KeyValue) trace.SpanStartOption {
	return trace.WithAttributes(attrs...)
}
