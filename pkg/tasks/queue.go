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

package tasks

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/hieunyit/tutti/pkg/entity"
)

func eventQueue(p ReplicationTask) string {
	return replicationQueueName(QueueEventsPrefix, p.GetReplicationID())
}
func initMigrationListQueue(p ReplicationTask) string {
	return replicationQueueName(QueueMigrateListObjectsPrefix, p.GetReplicationID())
}

func initMigrationCopyQueue(p ReplicationTask) string {
	return replicationQueueName(QueueMigrateCopyObjectPrefix, p.GetReplicationID())
}

type Queue string

const (
	QueueAPI                      Queue = "api"
	QueueMigrateListObjectsPrefix Queue = "migr_list_obj"
	QueueMigrateCopyObjectPrefix  Queue = "migr_copy_obj"
	QueueDiff                     Queue = "diff"
	QueueDiffFix                  Queue = "diff_fix"
	QueueEventsPrefix             Queue = "event"
)

// Priority defines the priority of the queues from highest to lowest.
// Live events (QueueEventsPrefix) are given high priority (80) over bulk copy (20)
// and bulk listing (10), preventing heavy bulk migrations from starving live replication traffic.
var Priority = map[string]int{
	string(QueueAPI):                             200, // highest priority for interactive operations
	string(QueueEventsPrefix) + ":*":             80,  // live replication events: allocated dominant share during concurrent bulk
	string(QueueDiffFix) + ":*":                  50,
	string(QueueDiff) + ":*":                     40,
	string(QueueMigrateCopyObjectPrefix) + ":*":  20,  // bulk migration copy
	string(QueueMigrateListObjectsPrefix) + ":*": 10,  // bulk listing (controlled by pagination & backpressure)
	"*":                                          1,   // fallback for legacy queues
}

// LivePriority defines priority for interactive API, live replication events, and diff fix queues.
// When live queues are idle, it also processes bulk queues as fallback.
var LivePriority = map[string]int{
	string(QueueAPI):                             200,
	string(QueueEventsPrefix) + ":*":             80,
	string(QueueDiffFix) + ":*":                  50,
	string(QueueDiff) + ":*":                     40,
	string(QueueMigrateCopyObjectPrefix) + ":*":  20,
	string(QueueMigrateListObjectsPrefix) + ":*": 10,
	"*":                                          1,
}

// BulkPriority defines dedicated priorities exclusively for bulk migration copy and listing tasks,
// ensuring bulk progress even when live event queues are continuously saturated.
var BulkPriority = map[string]int{
	string(QueueMigrateCopyObjectPrefix) + ":*":  20,
	string(QueueMigrateListObjectsPrefix) + ":*": 10,
	"*":                                          1,
}

var replicationQueuePrefixes = []Queue{QueueMigrateListObjectsPrefix, QueueMigrateCopyObjectPrefix, QueueEventsPrefix}

func replicationQueueName(queuePrefix Queue, id entity.UniversalReplicationID) string {
	if !slices.Contains(replicationQueuePrefixes, queuePrefix) {
		panic(fmt.Sprintf("%s is not a replication queue prefix", queuePrefix))
	}
	return fmt.Sprintf("%s:%s", queuePrefix, id.AsString())
}

func isReplicationQueue(queue string) bool {
	for _, prefix := range replicationQueuePrefixes {
		if strings.HasPrefix(queue, string(prefix)+":") {
			return true
		}
	}
	return false
}

func InitMigrationListQueue(id entity.UniversalReplicationID) string {
	return replicationQueueName(QueueMigrateListObjectsPrefix, id)
}

func InitMigrationCopyQueue(id entity.UniversalReplicationID) string {
	return replicationQueueName(QueueMigrateCopyObjectPrefix, id)
}

func EventMigrationQueue(id entity.UniversalReplicationID) string {
	return replicationQueueName(QueueEventsPrefix, id)
}

func AllReplicationQueues(id entity.UniversalReplicationID) []string {
	return []string{
		InitMigrationListQueue(id),
		InitMigrationCopyQueue(id),
		EventMigrationQueue(id),
	}
}

func DiffQueue(id entity.DiffID) string {
	return diffQueueName(QueueDiff, id)
}

func DiffFixQueue(id entity.DiffID) string {
	return diffQueueName(QueueDiffFix, id)
}

func diffQueueName(queuePrefix Queue, id entity.DiffID) string {
	switch queuePrefix {
	case QueueDiff, QueueDiffFix:
	default:
		panic(fmt.Sprintf("%s is not a diff queue prefix", queuePrefix))
	}

	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := json.NewEncoder(encoder).Encode(&id); err != nil {
		panic(fmt.Errorf("unable to encode diff id: %w", err))
	}
	if err := encoder.Close(); err != nil {
		panic(fmt.Errorf("unable to finalize encoding of diff id: %w", err))
	}

	return fmt.Sprintf("%s:%s", queuePrefix, buf.String())
}
