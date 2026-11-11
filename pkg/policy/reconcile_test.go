package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
)

type fakeReconcilePolicySvc struct {
	Service
	userRepls   map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended
	bucketRepls map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended
	resumed     []string
}

func (f *fakeReconcilePolicySvc) ListUserReplicationsInfo(context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return f.userRepls, nil
}

func (f *fakeReconcilePolicySvc) ListBucketReplicationsInfo(_ context.Context, u string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error) {
	out := map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{}
	for p, s := range f.bucketRepls {
		if p.User == u {
			out[p] = s
		}
	}
	return out, nil
}

func (f *fakeReconcilePolicySvc) ResumeReplication(_ context.Context, id entity.UniversalReplicationID) error {
	f.resumed = append(f.resumed, id.AsString())
	return nil
}

func TestReconcileReplications_ResumesPendingFailovers(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	pol1 := entity.UserReplicationPolicy{User: "alice", FromStorage: "s1", ToStorage: "s2"}
	pol2 := entity.BucketReplicationPolicy{User: "bob", FromStorage: "s1", ToStorage: "s2", FromBucket: "b1", ToBucket: "b2"}

	fake := &fakeReconcilePolicySvc{
		userRepls: map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended{
			pol1: {ReplicationStatus: &entity.ReplicationStatus{FailoverPending: true}},
		},
		bucketRepls: map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{
			pol2: {ReplicationStatus: &entity.ReplicationStatus{FailoverPending: true}},
		},
	}

	users := func() []string { return []string{"alice", "bob"} }

	res, err := ReconcileReplications(ctx, fake, users)
	r.NoError(err)
	r.Equal(2, res.Checked)
	r.Equal(2, res.Resumed)
	r.Equal(0, res.Errors)
	r.Contains(fake.resumed, entity.UniversalFromUserReplication(pol1).String())
	r.Contains(fake.resumed, entity.UniversalFromBucketReplication(pol2).String())
}
