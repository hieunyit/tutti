// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func Test_policySvc_SetBucketRouting(t *testing.T) {
	c := testutil.SetupRedis(t)
	type before struct {
		mainStorage                     string
		userRepl                        *entity.UserReplicationPolicy
		bucketRepl                      *entity.BucketReplicationPolicy
		userReplHasZeroDowntimeSwitch   bool
		bucketReplHasZeroDowntimeSwitch bool
		id                              *entity.BucketRoutingPolicyID
		toStorage                       string
	}
	tests := []struct {
		name      string
		before    before
		id        entity.BucketRoutingPolicyID
		toStorage string
		wantErr   error
	}{
		{
			name: "set routing without replications",
			before: before{
				mainStorage: "main",
			},
			id:        entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			toStorage: "storageA",
			wantErr:   nil,
		},
		{
			name: "fail when user-level replication exists",
			before: before{
				mainStorage: "main",
				userRepl: &entity.UserReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					ToStorage:   "storageB",
				},
			},
			id:        entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			toStorage: "main",
			wantErr:   dom.ErrInvalidArg,
		},
		{
			name: "fail when bucket-level replication exists",
			before: before{
				mainStorage: "main",
				bucketRepl: &entity.BucketReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					FromBucket:  "bucket1",
					ToStorage:   "storageC",
					ToBucket:    "bucket1",
				},
			},
			id:        entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			toStorage: "main",
			wantErr:   dom.ErrInvalidArg,
		},
		{
			name: "fail when user-level replication is in zero-downtime switch",
			before: before{
				mainStorage: "main",
				userRepl: &entity.UserReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					ToStorage:   "storageB",
				},
				userReplHasZeroDowntimeSwitch: true,
			},
			id:        entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			toStorage: "storageA",
			wantErr:   dom.ErrInvalidArg,
		},
		{
			name: "fail when bucket-level replication is in zero-downtime switch",
			before: before{
				mainStorage: "main",
				bucketRepl: &entity.BucketReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					FromBucket:  "bucket1",
					ToStorage:   "storageC",
					ToBucket:    "bucket1",
				},
				bucketReplHasZeroDowntimeSwitch: true,
			},
			id:        entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			toStorage: "storageA",
			wantErr:   dom.ErrInvalidArg,
		},
		{
			name: "update routing",
			before: before{
				mainStorage: "main",
				id: &entity.BucketRoutingPolicyID{
					User:   "user1",
					Bucket: "bucket1",
				},
				toStorage: "storageB",
			},
			id:        entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			toStorage: "storageA",
			wantErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.FlushAll(t.Context()).Err(); err != nil {
				t.Fatal(err)
			}
			qs := tasks.QueueServiceMock{
				Queues: map[string]int{},
				Paused: map[string]bool{},
			}
			r := require.New(t)
			svc := NewService(c, &qs, tt.before.mainStorage)
			ctx := t.Context()

			if tt.before.userRepl != nil {
				_, err := svc.AddUserReplicationPolicy(ctx, *tt.before.userRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.bucketRepl != nil {
				_, err := svc.AddBucketReplicationPolicy(ctx, *tt.before.bucketRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.userReplHasZeroDowntimeSwitch {
				startZeroDowntimeSwitch(t, svc, &qs, entity.UniversalFromUserReplication(*tt.before.userRepl))
			}
			if tt.before.bucketReplHasZeroDowntimeSwitch {
				startZeroDowntimeSwitch(t, svc, &qs, entity.UniversalFromBucketReplication(*tt.before.bucketRepl))
			}
			if tt.before.id != nil {
				err := svc.SetBucketRouting(ctx, *tt.before.id, tt.before.toStorage)
				r.NoError(err)
			}

			err := svc.SetBucketRouting(ctx, tt.id, tt.toStorage)
			if tt.wantErr != nil {
				r.ErrorIs(err, tt.wantErr)
			} else {
				r.NoError(err)

				res, err := svc.GetBucketRoutings(ctx, tt.id.User)
				r.NoError(err)
				r.Equal(tt.toStorage, res[tt.id.Bucket])
				if tt.before.id != nil && tt.before.toStorage != tt.toStorage {
					// check update
					r.NotEqual(tt.before.toStorage, res[tt.id.Bucket])
				}
			}
		})
	}
}
func Test_policySvc_DeleteBucketRouting(t *testing.T) {
	c := testutil.SetupRedis(t)
	type before struct {
		mainStorage                     string
		userRepl                        *entity.UserReplicationPolicy
		bucketRepl                      *entity.BucketReplicationPolicy
		bucketReplHasZeroDowntimeSwitch bool
		bucketRoute                     *entity.BucketRoutingPolicyID
		buckeRouteToStorage             string
	}
	tests := []struct {
		name    string
		before  before
		id      entity.BucketRoutingPolicyID
		wantErr error
	}{
		{
			name: "delete routing without dependent replications",
			before: before{
				mainStorage: "main",
				bucketRoute: &entity.BucketRoutingPolicyID{
					User:   "user1",
					Bucket: "bucket1",
				},
				buckeRouteToStorage: "storageA",
			},
			id:      entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			wantErr: nil,
		},
		{
			name: "fail when dependent bucket-level replication exists",
			before: before{
				mainStorage: "main",
				bucketRepl: &entity.BucketReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					FromBucket:  "bucket1",
					ToStorage:   "storageC",
					ToBucket:    "bucket1-replica",
				},
			},
			id:      entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			wantErr: dom.ErrInvalidArg,
		},
		{
			name: "fail when bucket-level replication is in zero-downtime switch",
			before: before{
				mainStorage: "main",
				bucketRepl: &entity.BucketReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					FromBucket:  "bucket1",
					ToStorage:   "storageC",
					ToBucket:    "bucket1",
				},
				bucketReplHasZeroDowntimeSwitch: true,
			},
			id:      entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			wantErr: dom.ErrInvalidArg,
		},
		{
			name: "delete idempotent",
			before: before{
				mainStorage: "main",
			},
			id:      entity.BucketRoutingPolicyID{User: "user1", Bucket: "bucket1"},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.FlushAll(t.Context()).Err(); err != nil {
				t.Fatal(err)
			}
			qs := tasks.QueueServiceMock{
				Queues: map[string]int{},
				Paused: map[string]bool{},
			}
			r := require.New(t)
			svc := NewService(c, &qs, tt.before.mainStorage)
			ctx := t.Context()

			if tt.before.userRepl != nil {
				_, err := svc.AddUserReplicationPolicy(ctx, *tt.before.userRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.bucketRepl != nil {
				_, err := svc.AddBucketReplicationPolicy(ctx, *tt.before.bucketRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.bucketReplHasZeroDowntimeSwitch {
				startZeroDowntimeSwitch(t, svc, &qs, entity.UniversalFromBucketReplication(*tt.before.bucketRepl))
			}
			if tt.before.bucketRoute != nil {
				err := svc.SetBucketRouting(ctx, *tt.before.bucketRoute, tt.before.buckeRouteToStorage)
				r.NoError(err)
			}

			err := svc.DeleteBucketRouting(ctx, tt.id)
			if tt.wantErr != nil {
				r.ErrorIs(err, tt.wantErr)
			} else {
				r.NoError(err)

				res, err := svc.GetBucketRoutings(ctx, tt.id.User)
				r.NoError(err)
				_, exists := res[tt.id.Bucket]
				r.False(exists)
			}
		})
	}
}

func Test_policySvc_SetUserRouting(t *testing.T) {
	c := testutil.SetupRedis(t)
	type before struct {
		mainStorage                   string
		userRepl                      *entity.UserReplicationPolicy
		bucketRepl                    *entity.BucketReplicationPolicy
		userReplHasZeroDowntimeSwitch bool
		userRoute                     *string
		toStorage                     string
	}
	tests := []struct {
		name      string
		before    before
		user      string
		toStorage string
		wantErr   error
	}{
		{
			name: "set routing without replications",
			before: before{
				mainStorage: "main",
			},
			user:      "user1",
			toStorage: "storageA",
			wantErr:   nil,
		},
		{
			name: "fail when user-level replication exists",
			before: before{
				mainStorage: "main",
				userRepl: &entity.UserReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					ToStorage:   "storageB",
				},
			},
			user:      "user1",
			toStorage: "storageA",
			wantErr:   dom.ErrInvalidArg,
		},
		{
			name: "succeed when bucket-level replication exists",
			before: before{
				mainStorage: "main",
				bucketRepl: &entity.BucketReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					FromBucket:  "bucket1",
					ToStorage:   "storageC",
					ToBucket:    "bucket1-replica",
				},
			},
			user:      "user1",
			toStorage: "storageA",
			wantErr:   nil,
		},
		{
			name: "fail when user-level replication is in zero-downtime switch",
			before: before{
				mainStorage: "main",
				userRepl: &entity.UserReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					ToStorage:   "storageB",
				},
				userReplHasZeroDowntimeSwitch: true,
			},
			user:      "user1",
			toStorage: "storageA",
			wantErr:   dom.ErrInvalidArg,
		},
		{
			name: "update routing",
			before: before{
				mainStorage: "main",
				userRoute:   stringPtr("user1"),
				toStorage:   "storageB",
			},
			user:      "user1",
			toStorage: "storageA",
			wantErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.FlushAll(t.Context()).Err(); err != nil {
				t.Fatal(err)
			}
			qs := tasks.QueueServiceMock{
				Queues: map[string]int{},
				Paused: map[string]bool{},
			}
			r := require.New(t)
			svc := NewService(c, &qs, tt.before.mainStorage)
			ctx := t.Context()

			if tt.before.userRepl != nil {
				_, err := svc.AddUserReplicationPolicy(ctx, *tt.before.userRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.bucketRepl != nil {
				_, err := svc.AddBucketReplicationPolicy(ctx, *tt.before.bucketRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.userReplHasZeroDowntimeSwitch {
				startZeroDowntimeSwitch(t, svc, &qs, entity.UniversalFromUserReplication(*tt.before.userRepl))
			}
			if tt.before.userRoute != nil {
				err := svc.SetUserRouting(ctx, *tt.before.userRoute, tt.before.toStorage)
				r.NoError(err)
			}

			err := svc.SetUserRouting(ctx, tt.user, tt.toStorage)
			if tt.wantErr != nil {
				r.ErrorIs(err, tt.wantErr)
			} else {
				r.NoError(err)

				res, err := svc.GetUserRoutings(ctx)
				r.NoError(err)
				r.Equal(tt.toStorage, res[tt.user])
				if tt.before.userRoute != nil && *tt.before.userRoute == tt.user && tt.before.toStorage != tt.toStorage {
					// check update
					r.NotEqual(tt.before.toStorage, res[tt.user])
				}
			}
		})
	}
}

func Test_policySvc_DeleteUserRouting(t *testing.T) {
	c := testutil.SetupRedis(t)
	type before struct {
		mainStorage                   string
		userRepl                      *entity.UserReplicationPolicy
		bucketRepl                    *entity.BucketReplicationPolicy
		userReplHasZeroDowntimeSwitch bool
		userRoute                     *string
		routeTo                       string
	}
	tests := []struct {
		name    string
		before  before
		user    string
		wantErr error
	}{
		{
			name: "delete routing without dependent replications",
			before: before{
				mainStorage: "main",
				userRoute:   stringPtr("user1"),
				routeTo:     "storageA",
			},
			user:    "user1",
			wantErr: nil,
		},
		{
			name: "fail when dependent user-level replication exists",
			before: before{
				mainStorage: "main",
				userRepl: &entity.UserReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					ToStorage:   "storageB",
				},
			},
			user:    "user1",
			wantErr: dom.ErrInvalidArg,
		},
		{
			name: "succeed when bucket-level replication exists",
			before: before{
				mainStorage: "main",
				userRoute:   stringPtr("user1"),
				routeTo:     "storageA",
				bucketRepl: &entity.BucketReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					FromBucket:  "bucket1",
					ToStorage:   "storageC",
					ToBucket:    "bucket1-replica",
				},
			},
			user:    "user1",
			wantErr: nil,
		},
		{
			name: "fail when user-level replication is in zero-downtime switch",
			before: before{
				mainStorage: "main",
				userRepl: &entity.UserReplicationPolicy{
					User:        "user1",
					FromStorage: "main",
					ToStorage:   "storageB",
				},
				userReplHasZeroDowntimeSwitch: true,
			},
			user:    "user1",
			wantErr: dom.ErrInvalidArg,
		},
		{
			name: "delete idempotent",
			before: before{
				mainStorage: "main",
			},
			user:    "user1",
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.FlushAll(t.Context()).Err(); err != nil {
				t.Fatal(err)
			}
			qs := tasks.QueueServiceMock{
				Queues: map[string]int{},
				Paused: map[string]bool{},
			}
			r := require.New(t)
			svc := NewService(c, &qs, tt.before.mainStorage)
			ctx := t.Context()

			if tt.before.userRepl != nil {
				_, err := svc.AddUserReplicationPolicy(ctx, *tt.before.userRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.bucketRepl != nil {
				_, err := svc.AddBucketReplicationPolicy(ctx, *tt.before.bucketRepl, entity.ReplicationOptions{})
				r.NoError(err)
			}
			if tt.before.userReplHasZeroDowntimeSwitch {
				startZeroDowntimeSwitch(t, svc, &qs, entity.UniversalFromUserReplication(*tt.before.userRepl))
			}
			if tt.before.userRoute != nil {
				err := svc.SetUserRouting(ctx, *tt.before.userRoute, tt.before.routeTo)
				r.NoError(err)
			}

			err := svc.DeleteUserRouting(ctx, tt.user)
			if tt.wantErr != nil {
				r.ErrorIs(err, tt.wantErr)
			} else {
				r.NoError(err)

				res, err := svc.GetUserRoutings(ctx)
				r.NoError(err)
				_, exists := res[tt.user]
				r.False(exists)
			}
		})
	}
}

func Test_policySvc_UpdateRouting(t *testing.T) {
	newSvc := func(t *testing.T) *policySvc {
		c := testutil.SetupRedis(t)
		return NewService(c, &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}, "main")
	}
	t.Run("user routing", func(t *testing.T) {
		r := require.New(t)
		svc := newSvc(t)
		ctx := t.Context()
		// missing routing -> NotFound, nothing created
		r.ErrorIs(svc.UpdateUserRouting(ctx, "u1", "b"), dom.ErrNotFound)
		routes, err := svc.GetUserRoutings(ctx)
		r.NoError(err)
		r.Empty(routes)

		r.NoError(svc.SetUserRouting(ctx, "u1", "a"))
		r.NoError(svc.UpdateUserRouting(ctx, "u1", "b"))
		routes, err = svc.GetUserRoutings(ctx)
		r.NoError(err)
		r.Equal(map[string]string{"u1": "b"}, routes)

		// block state is preserved
		r.NoError(svc.BlockUserRouting(ctx, "u1"))
		r.NoError(svc.UpdateUserRouting(ctx, "u1", "c"))
		blocks, err := svc.GetUserRoutingBlocks(ctx)
		r.NoError(err)
		r.True(blocks["u1"])
		routes, _ = svc.GetUserRoutings(ctx)
		r.Equal("c", routes["u1"])
	})
	t.Run("user routing conflicts with user replication", func(t *testing.T) {
		r := require.New(t)
		svc := newSvc(t)
		ctx := t.Context()
		r.NoError(svc.SetUserRouting(ctx, "u1", "a"))
		_, err := svc.AddUserReplicationPolicy(ctx, entity.UserReplicationPolicy{User: "u1", FromStorage: "a", ToStorage: "b"}, entity.ReplicationOptions{})
		r.NoError(err)
		r.ErrorIs(svc.UpdateUserRouting(ctx, "u1", "c"), dom.ErrInvalidArg)
		routes, _ := svc.GetUserRoutings(ctx)
		r.Equal("a", routes["u1"])
	})
	t.Run("bucket routing", func(t *testing.T) {
		r := require.New(t)
		svc := newSvc(t)
		ctx := t.Context()
		id := entity.BucketRoutingPolicyID{User: "u1", Bucket: "bk"}
		r.ErrorIs(svc.UpdateBucketRouting(ctx, id, "b"), dom.ErrNotFound)
		r.NoError(svc.SetBucketRouting(ctx, id, "a"))
		r.NoError(svc.UpdateBucketRouting(ctx, id, "b"))
		routes, err := svc.GetBucketRoutings(ctx, "u1")
		r.NoError(err)
		r.Equal(map[string]string{"bk": "b"}, routes)
		// other bucket has no routing
		r.ErrorIs(svc.UpdateBucketRouting(ctx, entity.BucketRoutingPolicyID{User: "u1", Bucket: "other"}, "b"), dom.ErrNotFound)
	})
	t.Run("bucket routing conflicts with bucket replication", func(t *testing.T) {
		r := require.New(t)
		svc := newSvc(t)
		ctx := t.Context()
		id := entity.BucketRoutingPolicyID{User: "u1", Bucket: "bk"}
		r.NoError(svc.SetBucketRouting(ctx, id, "a"))
		_, err := svc.AddBucketReplicationPolicy(ctx, entity.BucketReplicationPolicy{
			User: "u1", FromStorage: "a", FromBucket: "bk", ToStorage: "c", ToBucket: "bk2"}, entity.ReplicationOptions{})
		r.NoError(err)
		r.ErrorIs(svc.UpdateBucketRouting(ctx, id, "b"), dom.ErrInvalidArg)
		routes, _ := svc.GetBucketRoutings(ctx, "u1")
		r.Equal("a", routes["bk"])
	})
}
