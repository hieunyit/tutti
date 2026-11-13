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

package tasks

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
)

func Test_ReplicationGetterSetter(t *testing.T) {
	r := require.New(t)
	expect := BucketCreatePayload{
		Bucket:   "fb",
		Location: "l",
	}
	r.Empty(expect.ID.AsString())
	r.True(expect.ID.IsEmpty())

	expect.SetReplicationID(entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "u",
		FromStorage: "fs",
		FromBucket:  "fb",
		ToStorage:   "ts",
		ToBucket:    "tb",
	}), 0)
	r.NotEmpty(expect.ID.AsString())
	r.False(expect.ID.IsEmpty())
	r.EqualValues("u", expect.ID.User())
	r.EqualValues("fs", expect.ID.FromStorage())
	r.EqualValues("ts", expect.ID.ToStorage())
	fromBucket, toBucket := expect.ID.FromToBuckets(expect.Bucket)
	r.EqualValues("fb", fromBucket)
	r.EqualValues("tb", toBucket)
	r.EqualValues(expect.ID, expect.GetReplicationID())
}

func TestQueueNameForEmptyUserDistinct(t *testing.T) {
	empty := entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("", "s1", "s2"))
	alice := entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("alice", "s1", "s2"))
	for _, prefix := range replicationQueuePrefixes {
		qe := replicationQueueName(prefix, empty)
		qa := replicationQueueName(prefix, alice)
		if qe == qa {
			t.Fatalf("queue names collide: %q", qe)
		}
		if qe != string(prefix)+"::s1:s2" {
			t.Fatalf("unexpected queue name %q", qe)
		}
		if !isReplicationQueue(qe) {
			t.Fatalf("%q must be a replication queue", qe)
		}
		id, err := entity.UniversalIDFromString(strings.TrimPrefix(qe, string(prefix)+":"))
		if err != nil || id != empty {
			t.Fatalf("queue name does not parse back: %v %v", id, err)
		}
	}
}

// Diff payloads carry their own User string; an empty user (storage-level pair) encodes with a valid task id
// and queue and keeps the empty user through the JSON payload.
func TestDiffPayloadsEmptyUser(t *testing.T) {
	locs := []MigrateLocation{{Storage: "s1", Bucket: "b"}, {Storage: "s2", Bucket: "b"}}
	for _, payload := range []any{
		DiffPayload{Locations: locs},
		DiffListObjectsPayload{Locations: locs},
		DiffListVersionsPayload{Locations: locs},
		DiffFixCollectObjectsPayload{Locations: locs},
		DiffFixRemoveObjectsPayload{Locations: locs},
		DiffFixEnsureObjectsRemovedPayload{Locations: locs},
	} {
		task, err := EncodeAny(t.Context(), payload)
		if err != nil {
			t.Fatalf("%T: %v", payload, err)
		}
		var got struct{ User string }
		if err := json.Unmarshal(task.Payload(), &got); err != nil || got.User != "" {
			t.Fatalf("%T: user %q err %v", payload, got.User, err)
		}
	}
}
