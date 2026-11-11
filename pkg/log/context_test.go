package log

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	otrace "go.opentelemetry.io/otel/trace"
)

func TestStartNew_PreservesTraceContextAndDetachesCancellation(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tracer := tp.Tracer("test")
	parentCtx, parentCancel := context.WithCancel(context.Background())
	ctxWithSpan, span := tracer.Start(parentCtx, "parent-span")
	defer span.End()

	newCtx, newCancel := StartNew(ctxWithSpan)
	defer newCancel()

	// 1. Span context is preserved in newCtx
	newSpan := otrace.SpanFromContext(newCtx)
	require.True(t, newSpan.SpanContext().IsValid(), "span context must be valid in detached context")
	require.Equal(t, span.SpanContext().TraceID(), newSpan.SpanContext().TraceID(), "trace ID must match parent")
	require.Equal(t, span.SpanContext().SpanID(), newSpan.SpanContext().SpanID(), "span ID must match parent")

	// 2. Canceling parent context does NOT cancel newCtx
	parentCancel()
	require.Error(t, parentCtx.Err())
	require.NoError(t, newCtx.Err(), "new context must not be cancelled when parent is cancelled")

	// 3. Calling newCancel DOES cancel newCtx
	newCancel()
	require.Error(t, newCtx.Err(), "calling newCancel must cancel newCtx")
}
