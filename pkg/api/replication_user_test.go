package api

import (
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

type noUserCreds struct{ userCreds }

func (noUserCreds) ValidateReplicationID(entity.UniversalReplicationID) error { return dom.ErrNotFound }

func TestPolicy_AddReplicationRequiresExplicitExistingUser(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	policySvc := policy.NewService(rc, &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}, "main")
	h := PolicyHandlers(noUserCreds{}, nil, nil, policySvc, nil, nil, nil, nil, nil, store.NewUserLocker(rc, time.Second), nil)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrorInterceptor()))
	pb.RegisterPolicyServer(srv, h)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	c := pb.NewPolicyClient(conn)

	// unknown user: InvalidArgument too
	_, err = c.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: &pb.ReplicationID{User: "ghost", FromStorage: "main", ToStorage: "a"}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
