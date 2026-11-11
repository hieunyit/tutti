package util

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServer_ShutdownDeadline(t *testing.T) {
	s := NewServer()
	cleanupHasDeadline := make(chan bool, 1)

	err := s.Add("test_worker", func(ctx context.Context) error {
		// Wait until canceled
		<-ctx.Done()
		return ctx.Err()
	}, func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		cleanupHasDeadline <- ok
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case hasDeadline := <-cleanupHasDeadline:
		require.True(t, hasDeadline, "cleanup context should have a deadline")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cleanup to be called")
	}

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Serve to finish")
	}
}
