package syncer

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
	"github.com/hieunyit/tutti/service/worker/copy"
)

type fakeCopy struct {
	copy.CopySvc
	calls []copy.File
	err   error
}

func (f *fakeCopy) CopyObject(_ context.Context, _ string, from copy.File, to copy.File) error {
	f.calls = append(f.calls, from, to)
	return f.err
}

type failClients struct {
	objstore.Clients
	calls int
}

func (f *failClients) AsS3(context.Context, string, string) (s3client.Client, error) {
	f.calls++
	return nil, errors.New("no client")
}

func setup(t *testing.T) (*ObjectSyncer, *fakeCopy, *failClients, meta.VersionService, tasks.ObjectSyncPayload) {
	t.Helper()
	rc := testutil.SetupRedis(t)
	verSvc := meta.NewVersionService(rc)
	fc := &fakeCopy{}
	cl := &failClients{}
	s := New(verSvc, store.NewObjectLocker(rc, 0), fc, cl)
	id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "u", FromStorage: "main", FromBucket: "b", ToStorage: "follower", ToBucket: "b",
	})
	p := tasks.ObjectSyncPayload{Object: dom.Object{Bucket: "b", Name: "o"}}
	p.SetReplicationID(id, 0)
	return s, fc, cl, verSvc, p
}

func TestSync_CopyUpdatesDestinationVersionThenSkips(t *testing.T) {
	r := require.New(t)
	s, fc, _, verSvc, p := setup(t)
	ctx := t.Context()
	dest := meta.Destination{Storage: "main", Bucket: "b"}
	_, err := verSvc.IncrementObj(ctx, p.ID, p.Object, dest)
	r.NoError(err)

	r.NoError(s.Sync(ctx, p))
	r.Len(fc.calls, 2)
	r.Equal("main", fc.calls[0].Storage)
	r.Equal("follower", fc.calls[1].Storage)
	v, err := verSvc.GetObj(ctx, p.ID, p.Object)
	r.NoError(err)
	r.Equal(1, v.To)
	r.Equal(1, v.From)

	// second sync (e.g. worker running the enqueued task): from <= to, no copy
	r.NoError(s.Sync(ctx, p))
	r.Len(fc.calls, 2)
}

func TestSync_CopyErrorKeepsVersion(t *testing.T) {
	r := require.New(t)
	s, fc, _, verSvc, p := setup(t)
	ctx := t.Context()
	_, err := verSvc.IncrementObj(ctx, p.ID, p.Object, meta.Destination{Storage: "main", Bucket: "b"})
	r.NoError(err)
	fc.err = errors.New("boom")
	r.Error(s.Sync(ctx, p))
	v, err := verSvc.GetObj(ctx, p.ID, p.Object)
	r.NoError(err)
	r.Equal(0, v.To)
}

func TestSync_SourceMissingIsSkipped(t *testing.T) {
	r := require.New(t)
	s, fc, _, verSvc, p := setup(t)
	ctx := t.Context()
	_, err := verSvc.IncrementObj(ctx, p.ID, p.Object, meta.Destination{Storage: "main", Bucket: "b"})
	r.NoError(err)
	fc.err = dom.ErrNotFound
	r.NoError(s.Sync(ctx, p))
}

func TestSync_EmptyVersionsTakesDeletePath(t *testing.T) {
	r := require.New(t)
	s, fc, cl, _, p := setup(t)
	err := s.Sync(t.Context(), p)
	r.Error(err)
	r.False(errors.Is(err, asynq.SkipRetry), "unclassified client failures must retain retry")
	r.Equal(1, cl.calls)
	r.Empty(fc.calls)
}
