package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

func TestUsers_NotConfiguredAndAuthDisabled(t *testing.T) {
	ctx := context.Background()
	c, _, _ := resilienceClient(t)
	_, err := c.ListS3Users(ctx, &emptypb.Empty{})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = c.CreateS3User(ctx, &pb.CreateS3UserRequest{Name: "u"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	// no auth service (sign-in disabled): console user RPCs fail with a clear reason
	for _, call := range []func() error{
		func() error { _, err := c.ListConsoleUsers(ctx, &emptypb.Empty{}); return err },
		func() error {
			_, err := c.CreateConsoleUser(ctx, &pb.CreateConsoleUserRequest{Username: "u"})
			return err
		},
		func() error { _, err := c.DeleteConsoleUser(ctx, &pb.ConsoleUserName{Username: "u"}); return err },
	} {
		err := call()
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.Contains(t, err.Error(), "FailedPrecondition")
	}
}
