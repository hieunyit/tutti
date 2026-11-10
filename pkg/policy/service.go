/*
 * Copyright © 2024 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package policy

import (
	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type Service interface {
	ContextSvc
	RoutingSvc
	ReplicationSvc
	ReplicationSwitchSvc
}

func NewService(client redis.UniversalClient, queueSVC tasks.QueueService, mainStorage string) *policySvc {
	// every mutation of routing/replication/switch data through this client bumps the snapshot version
	// and notifies the proxies (see change_notify.go)
	installChangeNotifier(client)
	return &policySvc{
		client:                       client,
		mainStorage:                  mainStorage,
		userRoutingStore:             store.NewUserRoutingStore(client),
		bucketRoutingStore:           store.NewBucketRoutingStore(client),
		bucketReplicationPolicyStore: store.NewBucketReplicationPolicyStore(client),
		userReplicationPolicyStore:   store.NewUserReplicationPolicyStore(client),
		userReplicationSwitchStore:   store.NewUserReplicationSwitchStore(client),
		bucketReplicationSwitchStore: store.NewBucketReplicationSwitchStore(client),
		bucketReplicationStatusStore: store.NewBucketReplicationStatusStore(client),
		userReplicationStatusStore:   store.NewUserReplicationStatusStore(client),
		replicationGenerationStore:   store.NewReplicationGenerationStore(client),
		queueSvc:                     queueSVC,
	}
}

// WithControlPlane attaches a PostgreSQL control-plane store as the source of truth for routing and replication
// configuration, falling back to Redis on read errors and rejecting mutations when PostgreSQL is unavailable.
func (r *policySvc) WithControlPlane(cp controlplane.Store) *policySvc {
	r.cp = cp
	return r
}

func (r *policySvc) ControlPlane() controlplane.Store {
	return r.cp
}

type policySvc struct {
	queueSvc           tasks.QueueService
	userRoutingStore   *store.UserRoutingStore
	bucketRoutingStore *store.BucketRoutingStore

	bucketReplicationPolicyStore *store.BucketReplicationPolicyStore
	userReplicationPolicyStore   *store.UserReplicationPolicyStore
	bucketReplicationStatusStore *store.BucketReplicationStatusStore
	userReplicationStatusStore   *store.UserReplicationStatusStore
	replicationGenerationStore   *store.ReplicationGenerationStore

	userReplicationSwitchStore   *store.UserReplicationSwitchStore
	bucketReplicationSwitchStore *store.BucketReplicationSwitchStore

	mainStorage string
	client      redis.UniversalClient
	cp          controlplane.Store
	// snap is the optional in-memory snapshot of proxy request data, see EnableSnapshot.
	snap *snapshot
}
