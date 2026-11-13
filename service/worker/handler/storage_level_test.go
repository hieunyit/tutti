// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// userSpyClients records the user of every client lookup.
type userSpyClients struct {
	client s3client.Client
	mu     sync.Mutex
	users  []string
}

func (c *userSpyClients) AsS3(_ context.Context, _, user string) (s3client.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users = append(c.users, user)
	return c.client, nil
}
func (c *userSpyClients) AsCommon(context.Context, string, string) (objstore.Common, error) {
	return nil, errors.New("unused")
}
func (c *userSpyClients) AsSwift(context.Context, string, string) (*gophercloud.ServiceClient, error) {
	return nil, errors.New("unused")
}

// The seed of a storage-level replication lists every bucket of the connection account: the empty user is
// passed to the client registry and never reaches the access policy store.
func TestHandleMigrationS3User_EmptyUserListsAllBuckets(t *testing.T) {
	r := require.New(t)
	mr, err := miniredis.Run()
	r.NoError(err)
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	backend := s3mem.New()
	r.NoError(backend.CreateBucket("b1"))
	r.NoError(backend.CreateBucket("b2"))
	ts := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(ts.Close)
	client, err := s3client.NewClient(t.Context(), metrics.NewS3Service(false),
		s3.StorageAddress{Address: ts.URL, HttpTimeout: time.Minute},
		s3.CredentialsV4{AccessKeyID: "k", SecretAccessKey: "s"}, "from", "")
	r.NoError(err)

	clients := &userSpyClients{client: client}
	queue := &mockQueueRecorder{}
	s := &svc{clients: clients, queueSvc: queue, limit: ratelimit.New(rc, nil)}
	// a shared connection account whose policy lookup must never be consulted for the empty user
	s.SetBucketScope(&accesspolicy.Scope{
		Shared: func(string, string) bool { return true },
		Get: func(context.Context, string) (accesspolicy.Policy, error) {
			return accesspolicy.Policy{}, errors.New("policy must not be read for the empty user")
		},
	})

	id := entity.UniversalFromUserReplication(entity.UserReplicationPolicy{FromStorage: "from", ToStorage: "to"})
	r.True(id.IsStorageLevel())
	payload := tasks.MigrateS3UserPayload{}
	payload.SetReplicationID(id, 1)
	task, err := tasks.EncodeAny(t.Context(), payload)
	r.NoError(err)

	r.NoError(s.HandleMigrationS3User(t.Context(), task))
	r.Equal([]string{""}, clients.users)
	var buckets []string
	for _, tk := range queue.tasks {
		bc, ok := tk.(tasks.BucketCreatePayload)
		r.True(ok)
		r.Equal(id, bc.ID)
		buckets = append(buckets, bc.Bucket)
	}
	r.ElementsMatch([]string{"b1", "b2"}, buckets)
}
