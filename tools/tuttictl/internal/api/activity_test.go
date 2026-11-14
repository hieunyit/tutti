package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

func ev(cursor, typ, key string) *pb.ReplicationActivityEvent {
	return &pb.ReplicationActivityEvent{Cursor: cursor, Type: typ, Key: key, Time: timestamppb.New(time.Date(2026, 11, 15, 10, 0, 1, 0, time.UTC))}
}

func TestFormatActivityEvent(t *testing.T) {
	r := require.New(t)
	done := ev("1-0", "done", "photos/a.jpg")
	done.Size, done.DurationMs, done.BytesPerSecond = 12<<20, 1200, 10<<20
	r.Equal("2026-11-15T10:00:01Z DONE    photos/a.jpg 12.0MB 1.2s 10.0MB/s", FormatActivityEvent(done))

	failed := ev("2-0", "failed", "b")
	failed.Attempt, failed.Error = 3, "boom"
	r.Equal("2026-11-15T10:00:01Z FAILED  b attempt=3: boom", FormatActivityEvent(failed))
	r.Equal("2026-11-15T10:00:01Z STARTED k", FormatActivityEvent(ev("3-0", "started", "k")))
}

func TestPrintActivityFollowUsesCursor(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var afters []string
	calls := 0
	fetch := func(_ context.Context, after string, limit int32) (*pb.GetReplicationActivityResponse, error) {
		afters = append(afters, after)
		calls++
		switch calls {
		case 1:
			r.EqualValues(2, limit)
			return &pb.GetReplicationActivityResponse{Events: []*pb.ReplicationActivityEvent{ev("1-0", "started", "a"), ev("2-0", "done", "a")}, NextCursor: "2-0"}, nil
		case 2:
			return nil, errors.New("temporary")
		case 3:
			return &pb.GetReplicationActivityResponse{Events: []*pb.ReplicationActivityEvent{ev("3-0", "done", "b")}, NextCursor: "3-0"}, nil
		default:
			cancel()
			return &pb.GetReplicationActivityResponse{NextCursor: "3-0"}, nil
		}
	}
	var out, errOut bytes.Buffer
	r.NoError(PrintActivity(ctx, fetch, &out, &errOut, 2, true, time.Millisecond))
	r.Equal([]string{"", "2-0", "2-0", "3-0"}, afters)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	r.Len(lines, 3)
	r.Contains(lines[2], "DONE    b")
	r.Contains(errOut.String(), "retrying")
}

func TestPrintActivityNoFollow(t *testing.T) {
	r := require.New(t)
	fetch := func(context.Context, string, int32) (*pb.GetReplicationActivityResponse, error) {
		return &pb.GetReplicationActivityResponse{Events: []*pb.ReplicationActivityEvent{ev("1-0", "done", "a")}}, nil
	}
	var out bytes.Buffer
	r.NoError(PrintActivity(context.Background(), fetch, &out, io.Discard, 10, false, time.Second))
	r.Contains(out.String(), "DONE")
	r.Error(PrintActivity(context.Background(), func(context.Context, string, int32) (*pb.GetReplicationActivityResponse, error) {
		return nil, errors.New("x")
	}, &out, io.Discard, 10, false, time.Second))
}

func TestInvoke_GetReplicationActivity(t *testing.T) {
	r := require.New(t)
	type call struct{ method, path, body string }
	got := make(chan call, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		got <- call{req.Method, req.URL.Path, string(b)}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":[{"cursor":"1-0","type":"done","key":"a","size":"5"}],"nextCursor":"1-0"}`))
	}))
	defer srv.Close()
	conn, err := Connect(context.Background(), srv.URL, false)
	r.NoError(err)
	var res pb.GetReplicationActivityResponse
	req := &pb.GetReplicationActivityRequest{Id: &pb.ReplicationID{FromStorage: "A", ToStorage: "B"}, AfterCursor: "0-1", Limit: 50}
	r.NoError(conn.Invoke(context.Background(), pb.Policy_GetReplicationActivity_FullMethodName, req, &res))
	c := <-got
	r.Equal(http.MethodPost, c.method)
	r.Equal("/replication/activity", c.path)
	r.JSONEq(`{"id":{"fromStorage":"A","toStorage":"B"},"afterCursor":"0-1","limit":50}`, c.body)
	r.Equal("1-0", res.NextCursor)
	r.EqualValues(5, res.Events[0].Size)
}
