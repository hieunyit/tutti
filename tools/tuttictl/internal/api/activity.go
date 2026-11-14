package api

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

// FormatActivityEvent renders one activity event as a log line:
//
//	2026-11-15T10:00:01Z DONE    photos/a.jpg 12MB 1.2s 10MB/s
func FormatActivityEvent(e *pb.ReplicationActivityEvent) string {
	var b strings.Builder
	b.WriteString(e.GetTime().AsTime().UTC().Format("2006-01-02T15:04:05Z"))
	b.WriteByte(' ')
	fmt.Fprintf(&b, "%-7s", strings.ToUpper(e.GetType()))
	b.WriteByte(' ')
	b.WriteString(e.GetKey())
	if v := e.GetVersionId(); v != "" {
		b.WriteString(" (version " + v + ")")
	}
	if e.GetSize() > 0 {
		b.WriteString(" " + HumanBytes(e.GetSize()))
	}
	if e.GetType() == "done" || e.GetDurationMs() > 0 {
		b.WriteString(" " + (time.Duration(e.GetDurationMs()) * time.Millisecond).String())
	}
	if e.GetBytesPerSecond() > 0 {
		b.WriteString(" " + HumanBytes(e.GetBytesPerSecond()) + "/s")
	}
	if e.GetAttempt() > 1 {
		fmt.Fprintf(&b, " attempt=%d", e.GetAttempt())
	}
	if e.GetError() != "" {
		b.WriteString(": " + e.GetError())
	}
	return b.String()
}

// ActivityFetcher reads activity after a cursor (empty: newest events).
type ActivityFetcher func(ctx context.Context, after string, limit int32) (*pb.GetReplicationActivityResponse, error)

// PrintActivity prints the last tail events. With follow it then polls with the cursor every interval and prints
// only new events until ctx ends. A transient error while following is reported to errOut and retried.
func PrintActivity(ctx context.Context, fetch ActivityFetcher, out, errOut io.Writer, tail int, follow bool, interval time.Duration) error {
	if tail <= 0 || tail > 500 {
		tail = 500
	}
	res, err := fetch(ctx, "", int32(tail))
	if err != nil {
		return err
	}
	for _, e := range res.GetEvents() {
		fmt.Fprintln(out, FormatActivityEvent(e))
	}
	if !follow {
		return nil
	}
	cursor := res.GetNextCursor()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		res, err := fetch(ctx, cursor, 500)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Fprintln(errOut, "warning: unable to read activity, retrying:", FormatGrpcError(err))
			continue
		}
		for _, e := range res.GetEvents() {
			fmt.Fprintln(out, FormatActivityEvent(e))
		}
		cursor = res.GetNextCursor()
	}
}
