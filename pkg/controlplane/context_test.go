package controlplane

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestIsUnavailableIsAnAllowList(t *testing.T) {
	unavailable := []error{
		context.DeadlineExceeded,
		pgx.ErrTxClosed,
		io.EOF,
		&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
		&pgconn.PgError{Code: "08006"},
		&pgconn.PgError{Code: "57P01"},
		&pgconn.PgError{Code: "53300"},
		&pgconn.PgError{Code: "57014"},
		errors.New("closed pool"),
		fmt.Errorf("controlplane: begin: %w", syscall.ECONNRESET),
	}
	for _, e := range unavailable {
		require.True(t, IsUnavailable(e), "%v", e)
	}
	available := []error{
		nil,
		errors.New("some programming error"),
		&pgconn.PgError{Code: "23505"}, // unique violation
		&pgconn.PgError{Code: "42P01"}, // undefined table
		context.Canceled,
		dom.ErrNotFound,
		fmt.Errorf("%w: x", dom.ErrInvalidArg),
		ErrReadOnly,
		ErrVersionConflict,
	}
	for _, e := range available {
		require.False(t, IsUnavailable(e), "%v", e)
	}
	// a non-availability failure is not turned into 503
	plain := errors.New("syntax error")
	require.Equal(t, plain, WrapWriteError(plain))
	require.ErrorIs(t, WrapWriteError(io.EOF), dom.ErrUnavailable)
}
