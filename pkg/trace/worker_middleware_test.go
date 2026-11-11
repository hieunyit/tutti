package trace_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	otrace "go.opentelemetry.io/otel/trace"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/trace"
)

func TestTracePropagation_ProxyToWorker(t *testing.T) {
	r := require.New(t)

	// Set up TracerProvider and W3C Propagator
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 1. Simulate Proxy request starting a parent span
	proxyTracer := tp.Tracer("proxy_test")
	proxyCtx, proxySpan := proxyTracer.Start(context.Background(), "s3:put_object")
	defer proxySpan.End()

	parentTraceID := proxySpan.SpanContext().TraceID().String()
	parentSpanID := proxySpan.SpanContext().SpanID().String()
	r.NotEmpty(parentTraceID)

	// 2. Proxy detaches context using log.StartNew before enqueueing replication task
	replCtx, cancel := log.StartNew(proxyCtx)
	defer cancel()

	// Simulate client request finishing/canceling
	proxySpan.End()

	payload := tasks.ObjectSyncPayload{
		Object: tasks.ObjectSyncPayload{}.Object,
	}
	payload.SetReplicationID(entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "user1",
		FromStorage: "s1",
		ToStorage:   "s2",
		FromBucket:  "b1",
		ToBucket:    "b2",
	}), 1)

	task, err := tasks.EncodeAny(replCtx, &payload)
	r.NoError(err)

	// Verify that traceparent was injected into task payload even after log.StartNew
	traceparent, _ := payload.GetTraceContext()
	r.Contains(string(task.Payload()), parentTraceID)

	// 3. Worker executes task with trace.WorkerMiddleware
	var capturedWorkerTraceID string

	testHandler := asynq.HandlerFunc(func(taskCtx context.Context, t *asynq.Task) error {
		span := otrace.SpanFromContext(taskCtx)
		sc := span.SpanContext()
		capturedWorkerTraceID = sc.TraceID().String()
		return nil
	})

	mw := trace.WorkerMiddleware(tp)
	err = mw(testHandler).ProcessTask(context.Background(), task)
	r.NoError(err)

	// 4. Assert that worker span inherited parent TraceID
	r.Equal(parentTraceID, capturedWorkerTraceID)
	r.NotEmpty(parentSpanID)
	r.NotEmpty(traceparent)
}
