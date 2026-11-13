package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

func TestReplIDToString_EmptyUser(t *testing.T) {
	r := require.New(t)
	r.Equal("A -> B", ReplIDToString(&pb.ReplicationID{FromStorage: "A", ToStorage: "B"}))
	fb, tb := "b1", "b2"
	r.Equal("A:b1 -> B:b2", ReplIDToString(&pb.ReplicationID{FromStorage: "A", ToStorage: "B", FromBucket: &fb, ToBucket: &tb}))
	r.Equal("alice:A->B", ReplIDToString(&pb.ReplicationID{User: "alice", FromStorage: "A", ToStorage: "B"}))
}

func TestBuildAvailableBucketsLines(t *testing.T) {
	r := require.New(t)
	lines := AvailableBucketsLines(&pb.AvailableBucketsResponse{
		Buckets: []string{"z", "a"},
		CoveredBuckets: []*pb.CoveredBucket{
			{Bucket: "c", OwnerKind: "group", OwnerId: "g1"},
		},
	})
	r.Equal([]string{"a", "z", "", "covered (cannot be replicated):", "c\tgroup\tg1"}, lines)
	r.Equal([]string{"a"}, AvailableBucketsLines(&pb.AvailableBucketsResponse{Buckets: []string{"a"}}))
}

func TestFormatGrpcError_ConflictReason(t *testing.T) {
	r := require.New(t)
	body := []byte(`{"code":6,"message":"already exists","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"replication A->B conflicts with B->A"}]}`)
	err := errorFromResponse(http.StatusConflict, body)
	st, _ := status.FromError(err)
	r.Equal(codes.AlreadyExists, st.Code())
	r.Equal("replication A->B conflicts with B->A", FormatGrpcError(err))
}

func TestInvoke_AddReplicationOmitsEmptyUser(t *testing.T) {
	r := require.New(t)
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		got <- b
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()
	conn, err := Connect(context.Background(), srv.URL, false)
	r.NoError(err)
	req := &pb.AddReplicationRequest{Id: &pb.ReplicationID{FromStorage: "A", ToStorage: "B"}}
	r.NoError(conn.Invoke(context.Background(), pb.Policy_AddReplication_FullMethodName, req, &emptypb.Empty{}))
	r.JSONEq(`{"id":{"fromStorage":"A","toStorage":"B"}}`, string(<-got))
}
