// SPDX-License-Identifier: MIT
package observe

import (
	"context"
	"fmt"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Tracer struct {
	tracer trace.Tracer
}

func NewTracer(name string) *Tracer {
	return &Tracer{tracer: otel.Tracer(name)}
}

type Span struct {
	span trace.Span
}

func (t *Tracer) Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, *Span) {
	ctx, sp := t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, &Span{span: sp}
}

func (s *Span) End() {
	s.span.End()
}

func (s *Span) SetAttributes(kv ...attribute.KeyValue) {
	s.span.SetAttributes(kv...)
}

func (s *Span) RecordError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

func (s *Span) SetStatus(status codes.Code, description string) {
	s.span.SetStatus(status, description)
}

func SpanAddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

func SpanRecordDuration(ctx context.Context, name string, start time.Time, attrs ...attribute.KeyValue) {
	sp := trace.SpanFromContext(ctx)
	sp.AddEvent(name+".done", trace.WithAttributes(
		append(attrs, attribute.Int64("duration_ms", time.Since(start).Milliseconds()))...,
	))
}

func ErrAttr(err error) attribute.KeyValue {
	return attribute.String(strconst.StrError, err.Error())
}

func IntAttr(key string, val int) attribute.KeyValue {
	return attribute.Int(key, val)
}

func StrAttr(key, val string) attribute.KeyValue {
	return attribute.String(key, val)
}

func BoolAttr(key string, val bool) attribute.KeyValue {
	return attribute.Bool(key, val)
}

func StrSliceAttr(key string, val []string) attribute.KeyValue {
	return attribute.StringSlice(key, val)
}

func FormatProviderSpanName(provider, model string) string {
	return fmt.Sprintf("llm.%s.%s", provider, model)
}

func FormatStageSpanName(pipeline, stage string) string {
	return fmt.Sprintf("pipeline.%s.%s", pipeline, stage)
}
