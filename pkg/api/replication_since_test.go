// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

func TestAddReplicationSince(t *testing.T) {
	e := newSLEnv(t, allConn, "bkt")
	add := func(to string, since *timestamppb.Timestamp) error {
		_, err := e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{
			Id:   &pb.ReplicationID{FromStorage: "main", ToStorage: to, FromBucket: ptr("bkt"), ToBucket: ptr("bkt")},
			Opts: &pb.ReplicationOpts{Since: since},
		})
		return err
	}
	// future and pre-2000 cutoffs are rejected
	require.Equal(t, codes.InvalidArgument, status.Code(add("a", timestamppb.New(time.Now().Add(time.Hour)))))
	require.Equal(t, codes.InvalidArgument, status.Code(add("a", timestamppb.New(time.Unix(0, 0)))))

	since := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, add("a", timestamppb.New(since)))
	require.NoError(t, add("b", nil))

	list, err := e.client.ListReplications(t.Context(), &pb.ListReplicationsRequest{})
	require.NoError(t, err)
	got := map[string]*timestamppb.Timestamp{}
	for _, r := range list.Replications {
		got[r.Id.ToStorage] = r.Since
	}
	require.Equal(t, since, got["a"].AsTime())
	require.Nil(t, got["b"], "no cutoff: since is unset")
}

func ptr(s string) *string { return &s }
