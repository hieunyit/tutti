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
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

type QueueService interface {
	UnprocessedCount(ctx context.Context, ignoreNotFound bool, queues ...string) (int, error)
	IsPaused(ctx context.Context, queueName string) (bool, error)
	Resume(ctx context.Context, queueName string) error
	Pause(ctx context.Context, queueName string) error
	Delete(ctx context.Context, queueName string, force bool) error
	// Stop pauses queues, cancels active tasks and waits until asynq reports none active.
	// Handlers of canceled tasks may still be finishing. Queues stay paused.
	Stop(ctx context.Context, queues ...string) error
	// Purge requires stopped queues: asynq refuses to delete a queue with active tasks.
	Purge(ctx context.Context, queues ...string) error
	Stats(ctx context.Context, queueName string) (*QueueStats, error)
	EnqueueTask(ctx context.Context, task any, opts ...asynq.Option) error
}

// BatchEnqueuer is an optional capability of QueueService: enqueue many tasks in as few
// Redis round trips as possible while keeping the broker admission semantics of EnqueueTask.
type BatchEnqueuer interface {
	// EnqueueTasks enqueues tasks in order. It returns how many leading tasks were accepted
	// (a task whose ID already exists counts as accepted, like EnqueueTask). If the broker
	// rejects a task by admission, enqueueing stops and the error is *dom.ErrRateLimitExceeded,
	// exactly as EnqueueTask reports it; tasks after the rejected one are not enqueued.
	EnqueueTasks(ctx context.Context, tasks []any) (int, error)
}

// EnqueueMany enqueues tasks through BatchEnqueuer when svc supports it, otherwise one by one.
func EnqueueMany(ctx context.Context, svc QueueService, payloads []any) (int, error) {
	if b, ok := svc.(BatchEnqueuer); ok {
		return b.EnqueueTasks(ctx, payloads)
	}
	for i, p := range payloads {
		if err := svc.EnqueueTask(ctx, p); err != nil {
			return i, err
		}
	}
	return len(payloads), nil
}

// DefaultUnprocessedCountTTL bounds how long a queue size may be served from cache.
const DefaultUnprocessedCountTTL = 300 * time.Millisecond

type countEntry struct {
	count int
	at    time.Time
}

type QueueStats struct {
	// Current number of tasks awaiting or currently in processing in the queue.
	// Includes pending, active, scheduled, retry, and aggregating tasks.
	// Does NOT include archived (permanently failed / SkipRetry) tasks.
	Unprocessed int
	// Total number of times tasks in this queue finished processing, counting
	// every retry and archival. asynq bumps this counter on success AND on each
	// failed attempt, so it is NOT the number of successful tasks on its own.
	// Successful tasks = ProcessedTotal - FailedTotal.
	ProcessedTotal int
	// Total number of failed processing attempts (retries + archived tasks).
	FailedTotal int
	// Total number of archived (permanently failed / SkipRetry) tasks.
	Archived int

	// Paused indicates whether the queue is paused.
	// If true, tasks in the queue will not be processed.
	Paused bool

	// Total number of bytes that the queue and its tasks require to be stored in redis.
	// It is an approximate memory usage value in bytes since the value is computed by sampling.
	MemoryUsage int64

	// Latency of the queue, measured by the oldest pending task in the queue.
	Latency time.Duration
}

func activeWaitingCount(info *asynq.QueueInfo) int {
	if info == nil {
		return 0
	}
	return info.Pending + info.Active + info.Scheduled + info.Retry + info.Aggregating
}

func extractRedisFromInspector(insp *asynq.Inspector) redis.Cmdable {
	if insp == nil {
		return nil
	}
	return insp.RedisClient()
}

func NewQueueService(client *asynq.Client, inspector *asynq.Inspector) *queueService {
	return &queueService{
		inspector: inspector,
		client:    client,
		rc:        extractRedisFromInspector(inspector),
		countTTL:  DefaultUnprocessedCountTTL,
		now:       time.Now,
		counts:    map[string]countEntry{},
	}
}

// SetUnprocessedCountCache configures the cache TTL (<=0 disables caching) and, if now is
// not nil, the clock. Intended for tests and tuning; call before use.
func (q *queueService) SetUnprocessedCountCache(ttl time.Duration, now func() time.Time) {
	q.countMu.Lock()
	defer q.countMu.Unlock()
	q.countTTL = ttl
	if now != nil {
		q.now = now
	}
	q.counts = map[string]countEntry{}
}

var _ QueueService = (*queueService)(nil)

type queueService struct {
	inspector *asynq.Inspector
	client    *asynq.Client
	rc        redis.Cmdable

	countMu  sync.Mutex
	countTTL time.Duration
	now      func() time.Time
	counts   map[string]countEntry
}

func (q *queueService) cachedCount(queue string) (int, bool) {
	q.countMu.Lock()
	defer q.countMu.Unlock()
	if q.countTTL <= 0 {
		return 0, false
	}
	e, ok := q.counts[queue]
	if !ok || q.now().Sub(e.at) >= q.countTTL || q.now().Before(e.at) {
		return 0, false
	}
	return e.count, true
}

func (q *queueService) storeCount(queue string, count int) {
	q.countMu.Lock()
	defer q.countMu.Unlock()
	if q.countTTL > 0 {
		q.counts[queue] = countEntry{count: count, at: q.now()}
	}
}

// noteEnqueued optimistically bumps a cached count; it keeps the original timestamp so the
// TTL still bounds staleness. Never creates entries.
func (q *queueService) noteEnqueued(queue string, n int) {
	if n <= 0 {
		return
	}
	q.countMu.Lock()
	defer q.countMu.Unlock()
	if e, ok := q.counts[queue]; ok {
		e.count += n
		q.counts[queue] = e
	}
}

func (q *queueService) RedisClient() redis.Cmdable {
	if q.rc != nil {
		return q.rc
	}
	if q.inspector != nil {
		q.rc = extractRedisFromInspector(q.inspector)
		return q.rc
	}
	return nil
}

// WithStrictWatermark is retained for source compatibility. Admission is now
// strict for every caller of a queue with a configured high watermark.
// Deprecated: no context marker is needed by the atomic broker admission.
func WithStrictWatermark(ctx context.Context) context.Context { return ctx }

// Admission is enforced by the broker in the same Lua script that creates the
// task. A caller delayed between encoding and enqueue cannot commit over quota.
func (q *queueService) EnqueueTask(ctx context.Context, task any, opts ...asynq.Option) error {
	var queue string
	if len(opts) == 0 {
		if t, encErr := EncodeAny(ctx, task); encErr == nil {
			queue = t.QueueName()
			_, err := q.client.EnqueueContext(ctx, t)
			return q.finishEnqueue(queue, err)
		}
	}
	return q.finishEnqueue("", enqueueAny(ctx, q.client, task, opts...))
}

func (q *queueService) finishEnqueue(queue string, err error) error {
	if errors.Is(err, asynq.ErrDuplicateTask) || errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	if errors.Is(err, asynq.ErrQueueFull) {
		return &dom.ErrRateLimitExceeded{RetryIn: time.Second}
	}
	if err == nil && queue != "" {
		q.noteEnqueued(queue, 1)
	}
	return err
}

var _ BatchEnqueuer = (*queueService)(nil)

// EnqueueTasks enqueues tasks with one Redis round trip per run of same-queue tasks.
func (q *queueService) EnqueueTasks(ctx context.Context, payloads []any) (int, error) {
	encoded := make([]*asynq.Task, 0, len(payloads))
	for _, p := range payloads {
		t, err := EncodeAny(ctx, p)
		if err != nil {
			return 0, err
		}
		encoded = append(encoded, t)
	}
	n, err := q.client.EnqueueBatch(ctx, encoded)
	for _, t := range encoded[:n] {
		q.noteEnqueued(t.QueueName(), 1)
	}
	if errors.Is(err, asynq.ErrQueueFull) {
		return n, &dom.ErrRateLimitExceeded{RetryIn: time.Second}
	}
	return n, err
}

func (q *queueService) Stats(ctx context.Context, queueName string) (*QueueStats, error) {
	info, err := q.getInfo(ctx, queueName)
	if err != nil {
		return nil, err
	}
	return &QueueStats{
		Unprocessed:    activeWaitingCount(info),
		ProcessedTotal: info.ProcessedTotal,
		FailedTotal:    info.FailedTotal,
		Archived:       info.Archived,
		Paused:         info.Paused,
		MemoryUsage:    info.MemoryUsage,
		Latency:        info.Latency,
	}, nil
}

func (q *queueService) Delete(ctx context.Context, queueName string, force bool) error {
	err := q.inspector.DeleteQueue(queueName, force)
	if errors.Is(err, asynq.ErrQueueNotFound) {
		return fmt.Errorf("unable to delete queue %s: %w", queueName, dom.ErrNotFound)
	}
	if errors.Is(err, asynq.ErrQueueNotEmpty) {
		return fmt.Errorf("unable to delete non-empty queue %s: %w", queueName, dom.ErrInvalidArg)
	}
	return err
}

const (
	// a task held by a dead worker stays active until its asynq lease expires, so waiting longer is pointless
	stopTimeout      = 10 * time.Second
	stopPollInterval = 50 * time.Millisecond
)

func (q *queueService) Stop(ctx context.Context, queues ...string) error {
	// pause sets the flag even for missing queues, so later enqueued tasks are not processed
	for _, queue := range queues {
		if err := q.Pause(ctx, queue); err != nil {
			return fmt.Errorf("unable to pause queue %s: %w", queue, err)
		}
	}
	deadline := time.Now().Add(stopTimeout)
	for {
		// a task dequeued concurrently with pause may become active after the previous listing
		active, err := q.cancelActive(queues)
		if err != nil {
			return err
		}
		if active == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %d tasks are still active after %s, queues are paused, retry later", dom.ErrInternal, active, stopTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(stopPollInterval):
		}
	}
}

func (q *queueService) cancelActive(queues []string) (int, error) {
	const pageSize = 1000
	active := 0
	for _, queue := range queues {
		for page := 1; ; page++ {
			list, err := q.inspector.ListActiveTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
			if errors.Is(err, asynq.ErrQueueNotFound) {
				break
			}
			if err != nil {
				return 0, fmt.Errorf("unable to list active tasks of queue %s: %w", queue, err)
			}
			for _, task := range list {
				if err := q.inspector.CancelProcessing(task.ID); err != nil {
					return 0, fmt.Errorf("unable to cancel task %s of queue %s: %w", task.ID, queue, err)
				}
			}
			active += len(list)
			if len(list) < pageSize {
				break
			}
		}
	}
	return active, nil
}

func (q *queueService) Purge(ctx context.Context, queues ...string) error {
	for _, queue := range queues {
		err := q.Delete(ctx, queue, true)
		if errors.Is(err, dom.ErrNotFound) {
			// missing queue may still have the paused flag
			if err := q.Resume(ctx, queue); err != nil {
				return fmt.Errorf("unable to remove paused flag of queue %s: %w", queue, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("unable to delete queue %s: %w", queue, err)
		}
	}
	return nil
}

func StopAndPurgeReplicationQueues(ctx context.Context, svc QueueService, id entity.UniversalReplicationID) error {
	queues := AllReplicationQueues(id)
	if err := svc.Stop(ctx, queues...); err != nil {
		return fmt.Errorf("unable to stop replication queues: %w", err)
	}
	if err := svc.Purge(ctx, queues...); err != nil {
		return fmt.Errorf("unable to purge replication queues: %w", err)
	}
	return nil
}

func (q *queueService) UnprocessedCount(ctx context.Context, ignoreNotFound bool, queues ...string) (int, error) {
	if len(queues) == 0 {
		return 0, fmt.Errorf("%w: no queues specified", dom.ErrInvalidArg)
	}
	count := 0
	for _, queue := range queues {
		if c, ok := q.cachedCount(queue); ok {
			count += c
			continue
		}
		info, err := q.getInfo(ctx, queue)
		if ignoreNotFound && errors.Is(err, dom.ErrNotFound) {
			continue
		}
		if err != nil {
			return 0, err
		}
		c := activeWaitingCount(info)
		q.storeCount(queue, c)
		count += c
	}
	return count, nil
}

func (q *queueService) IsPaused(ctx context.Context, queueName string) (bool, error) {
	info, err := q.getInfo(ctx, queueName)
	if err != nil {
		return false, err
	}
	return info.Paused, nil
}

func (q *queueService) getInfo(_ context.Context, queueName string) (*asynq.QueueInfo, error) {
	info, err := q.inspector.GetQueueInfo(queueName)
	if err != nil {
		if errors.Is(err, asynq.ErrQueueNotFound) {
			return nil, fmt.Errorf("%w: queue %s does not exist", dom.ErrNotFound, queueName)
		}
		return nil, fmt.Errorf("get queue info for %s: %w", queueName, err)
	}
	return info, nil
}

func (q *queueService) Pause(ctx context.Context, queueName string) error {
	err := q.inspector.PauseQueue(queueName)
	if err != nil {
		// ignore error if queue is already paused
		if strings.HasSuffix(err.Error(), "is already paused") {
			return nil
		}
	}
	return err
}

func (q *queueService) Resume(ctx context.Context, queueName string) error {
	err := q.inspector.UnpauseQueue(queueName)
	if err != nil {
		// ignore error if queue is not paused
		if strings.HasSuffix(err.Error(), "is not paused") {
			return nil
		}
	}
	return err
}
