// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func Test_ReplicationRescanInterval_RedisRoundTrip(t *testing.T) {
	c := testutil.SetupRedis(t)
	qs := tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	svc := NewService(c, &qs, "main")
	ctx := t.Context()
	r := require.New(t)

	bid := entity.BucketReplicationPolicy{FromStorage: "main", ToStorage: "follower", FromBucket: "b", ToBucket: "b"}
	uid := entity.UserReplicationPolicy{User: "u", FromStorage: "main", ToStorage: "follower"}
	_, err := svc.AddBucketReplicationPolicy(ctx, bid, entity.ReplicationOptions{RescanInterval: time.Hour})
	r.NoError(err)
	_, err = svc.AddUserReplicationPolicy(ctx, uid, entity.ReplicationOptions{})
	r.NoError(err)
	bu := entity.UniversalFromBucketReplication(bid)
	uu := entity.UniversalFromUserReplication(uid)

	info, err := svc.GetReplicationPolicyInfoExtended(ctx, bu)
	r.NoError(err)
	r.Equal(time.Hour, info.RescanInterval())
	r.Equal(time.Hour, info.Rescan.Interval)
	info, err = svc.GetReplicationPolicyInfoExtended(ctx, uu)
	r.NoError(err)
	r.Zero(info.RescanInterval(), "unset means off")

	r.NoError(svc.SetReplicationRescanInterval(ctx, bu, 15*time.Minute))
	r.NoError(svc.RenameReplication(ctx, bu, "x"))
	info, _ = svc.GetReplicationPolicyInfoExtended(ctx, bu)
	r.Equal(15*time.Minute, info.RescanInterval(), "rename keeps it")
	r.NoError(svc.SetReplicationRescanInterval(ctx, uu, 2*time.Hour))
	info, _ = svc.GetReplicationPolicyInfoExtended(ctx, uu)
	r.Equal(2*time.Hour, info.RescanInterval())

	r.NoError(svc.SetReplicationRescanInterval(ctx, bu, 0))
	info, _ = svc.GetReplicationPolicyInfoExtended(ctx, bu)
	r.Zero(info.RescanInterval())
	r.Zero(info.Rescan.Interval)

	missing := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{FromStorage: "main", ToStorage: "follower", FromBucket: "none", ToBucket: "none"})
	r.ErrorIs(svc.SetReplicationRescanInterval(ctx, missing, time.Hour), dom.ErrNotFound)
}

func Test_ListRescanTargets_FiltersByStateAndConfig(t *testing.T) {
	c := testutil.SetupRedis(t)
	qs := tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	svc := NewService(c, &qs, "main")
	ctx := t.Context()
	r := require.New(t)
	add := func(to string, interval time.Duration) entity.UniversalReplicationID {
		id := entity.BucketReplicationPolicy{FromStorage: "main", ToStorage: to, FromBucket: "b", ToBucket: "b"}
		_, err := svc.AddBucketReplicationPolicy(ctx, id, entity.ReplicationOptions{RescanInterval: interval})
		r.NoError(err)
		u := entity.UniversalFromBucketReplication(id)
		qs.InitReplicationDone(u)
		return u
	}
	ok := add("ok", time.Hour)
	off := add("off", 0)
	_ = off
	notDone := add("notdone", time.Hour)
	qs.InitReplicationInProgress(notDone)
	paused := add("paused", time.Hour)
	qs.Paused[tasks.InitMigrationListQueue(paused)] = true
	archived := add("archived", time.Hour)
	bid, _ := archived.AsBucketID()
	r.NoError(svc.DeleteBucketReplication(ctx, bid))

	targets, err := svc.ListRescanTargets(ctx, []string{""})
	r.NoError(err)
	r.Len(targets, 1)
	r.Equal(ok, targets[0].ID)
	r.Equal(time.Hour, targets[0].Interval)
	r.NotZero(targets[0].Generation)
	r.False(targets[0].CreatedAt.IsZero())
}
