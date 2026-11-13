package api

import (
	"context"
	"testing"

	"github.com/hieunyit/tutti/pkg/dom"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/stretchr/testify/require"
)

func storageLevelID() *pb.ReplicationID {
	b := "b1"
	return &pb.ReplicationID{FromStorage: "A", ToStorage: "B", FromBucket: &b, ToBucket: &b}
}

func TestSwitch_RejectedForStorageLevelReplication(t *testing.T) {
	h := &policyHandlers{}
	ctx := context.Background()
	_, err := h.SwitchWithDowntime(ctx, &pb.SwitchDowntimeRequest{ReplicationId: storageLevelID()})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "protection group switchover")
	_, err = h.SwitchWithZeroDowntime(ctx, &pb.SwitchZeroDowntimeRequest{ReplicationId: storageLevelID()})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "protection group switchover")
}

func TestRouting_RequiresUser(t *testing.T) {
	h := &policyHandlers{}
	_, err := h.AddRouting(context.Background(), &pb.AddRoutingRequest{ToStorage: "B"})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	require.Contains(t, err.Error(), "user is required")
}
