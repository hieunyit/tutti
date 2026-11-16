// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func Test_ReplicationSince_RedisRoundTrip(t *testing.T) {
	c := testutil.SetupRedis(t)
	qs := tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	svc := NewService(c, &qs, "main")
	ctx := t.Context()
	r := require.New(t)
	since := time.Date(2026, 10, 17, 14, 3, 5, 0, time.UTC)

	bid := entity.BucketReplicationPolicy{FromStorage: "main", ToStorage: "follower", FromBucket: "b", ToBucket: "b"}
	uid := entity.UserReplicationPolicy{User: "u", FromStorage: "main", ToStorage: "follower"}
	_, err := svc.AddBucketReplicationPolicy(ctx, bid, entity.ReplicationOptions{Since: &since})
	r.NoError(err)
	_, err = svc.AddUserReplicationPolicy(ctx, uid, entity.ReplicationOptions{})
	r.NoError(err)

	got, err := svc.GetReplicationSince(ctx, entity.UniversalFromBucketReplication(bid))
	r.NoError(err)
	r.NotNil(got)
	r.True(since.Equal(*got))
	info, err := svc.GetReplicationPolicyInfoExtended(ctx, entity.UniversalFromBucketReplication(bid))
	r.NoError(err)
	r.True(since.Equal(*info.Since))

	got, err = svc.GetReplicationSince(ctx, entity.UniversalFromUserReplication(uid))
	r.NoError(err)
	r.Nil(got, "no cutoff: copy everything")

	// rename keeps it
	r.NoError(svc.RenameReplication(ctx, entity.UniversalFromBucketReplication(bid), "x"))
	got, _ = svc.GetReplicationSince(ctx, entity.UniversalFromBucketReplication(bid))
	r.True(since.Equal(*got))
}
