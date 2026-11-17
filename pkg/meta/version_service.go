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

package meta

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

type Destination struct {
	Storage string
	Bucket  string
}

func (d Destination) Validate() error {
	if d.Storage == "" {
		return fmt.Errorf("%w: empty storage in destination", dom.ErrInvalidArg)
	}
	if d.Bucket == "" {
		return fmt.Errorf("%w: empty bucket in destination", dom.ErrInvalidArg)
	}
	return nil
}

func (d Destination) IsFrom(replID entity.UniversalReplicationID) (bool, error) {
	fromBucket, toBucket := replID.FromToBuckets(d.Bucket)

	if d.Storage == replID.FromStorage() && fromBucket == d.Bucket {
		return true, nil
	}
	if d.Storage == replID.ToStorage() && toBucket == d.Bucket {
		return false, nil
	}
	return false, fmt.Errorf("%w: destination storage/bucket %s/%s does not match replication ID %s", dom.ErrInternal, d.Storage, d.Bucket, replID.AsString())
}

type Version struct {
	From int
	To   int
}

func (v Version) IsEmpty() bool {
	return v.From == 0 && v.To == 0
}

type VersionService interface {
	Cleanup(ctx context.Context, replID entity.UniversalReplicationID) error

	GetObj(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) (Version, error)
	IncrementObj(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination) (int, error)
	UpdateIfGreater(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination, version int) error
	DeleteObjAll(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) error
	DeleteObjWithDeadline(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, deadline time.Time) (time.Time, error)
	GetOrSetDeleteDeadline(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, deadline time.Time) (time.Time, error)

	GetACL(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) (Version, error)
	IncrementACL(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination) (int, error)
	UpdateACLIfGreater(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination, version int) error

	GetTags(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) (Version, error)
	IncrementTags(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination) (int, error)
	UpdateTagsIfGreater(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination, version int) error

	GetBucket(ctx context.Context, replID entity.UniversalReplicationID, bucket string) (Version, error)
	IncrementBucket(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination) (int, error)
	UpdateBucketIfGreater(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination, version int) error
	DeleteBucketAll(ctx context.Context, replID entity.UniversalReplicationID, bucket string) error

	GetBucketACL(ctx context.Context, replID entity.UniversalReplicationID, bucket string) (Version, error)
	IncrementBucketACL(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination) (int, error)
	UpdateBucketACLIfGreater(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination, version int) error

	GetBucketTags(ctx context.Context, replID entity.UniversalReplicationID, bucket string) (Version, error)
	IncrementBucketTags(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination) (int, error)
	UpdateBucketTagsIfGreater(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination, version int) error

	GCObjectTombstones(ctx context.Context, maxAge time.Duration) (int, error)
}

func NewVersionService(client redis.UniversalClient) VersionService {
	return &versionSvc{client: client}
}

type versionSvc struct {
	client redis.UniversalClient
}

func (s *versionSvc) GetObj(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) (Version, error) {
	return s.getObjVersion(ctx, replID, obj, payload)
}

func (s *versionSvc) IncrementObj(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination) (int, error) {
	keys, err := toRedisObjKeys(replID, obj, payload)
	if err != nil {
		return 0, err
	}
	if err := dest.Validate(); err != nil {
		return 0, err
	}
	isFrom, err := dest.IsFrom(replID)
	if err != nil {
		return 0, err
	}
	if isFrom {
		return s.incObjectSourceVersion(ctx, keys)
	}
	return s.incVersion(ctx, keys.hashKey, keys.toField, keys.fromField)
}

func (s *versionSvc) UpdateIfGreater(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination, version int) error {
	keys, err := toRedisObjKeys(replID, obj, payload)
	if err != nil {
		return err
	}
	return s.setDestIfGreater(ctx, replID, keys, dest, int(version))
}

func (s *versionSvc) DeleteObjAll(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) error {
	_, err := s.DeleteObjWithDeadline(ctx, replID, obj, time.Time{})
	return err
}

const extendDeleteDeadline = `
local previous = redis.call('HGET', KEYS[1], ARGV[1])
if previous and tonumber(previous) >= tonumber(ARGV[2]) then return previous end
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
return ARGV[2]`

// DeleteObjWithDeadline records the deadline alongside version deletion.
// A new delete event cannot shorten an existing delay.
func (s *versionSvc) DeleteObjWithDeadline(ctx context.Context, id entity.UniversalReplicationID, obj dom.Object, deadline time.Time) (time.Time, error) {
	key, fields, err := allRedisObjKeys(id, obj)
	if err != nil {
		return time.Time{}, err
	}
	rk := toRedisKeys(id, obj.Bucket, objectID(obj), payload)
	deadlineField := "deadline:" + rk.fromField[2:]
	now := time.Now().Unix()
	pipe := s.client.TxPipeline()
	pipe.HDel(ctx, key, fields...)
	for _, kind := range kinds {
		k := toRedisKeys(id, obj.Bucket, objectID(obj), kind)
		pipe.HSet(ctx, key, "ts:"+k.fromField[2:], now)
	}
	if deadline.IsZero() {
		_, err := pipe.Exec(ctx)
		return time.Time{}, err
	}
	saved := pipe.Eval(ctx, extendDeleteDeadline, []string{key}, deadlineField, deadline.UnixMilli())
	if _, err := pipe.Exec(ctx); err != nil {
		return time.Time{}, err
	}
	n, err := saved.Int64()
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(n), nil
}

var getOrSetDeleteDeadline = redis.NewScript(`
local deadline = redis.call('HGET', KEYS[1], ARGV[1])
if deadline then return deadline end
if ARGV[3] == '' then return false end
redis.call('HSET', KEYS[1], ARGV[1], ARGV[3], ARGV[2], ARGV[4])
return ARGV[3]`)

// GetOrSetDeleteDeadline also protects legacy missing metadata for which no delete event was recorded.
func (s *versionSvc) GetOrSetDeleteDeadline(ctx context.Context, id entity.UniversalReplicationID, obj dom.Object, deadline time.Time) (time.Time, error) {
	rk, err := toRedisObjKeys(id, obj, payload)
	if err != nil {
		return time.Time{}, err
	}
	proposed := ""
	if !deadline.IsZero() {
		proposed = strconv.FormatInt(deadline.UnixMilli(), 10)
	}
	suffix := rk.fromField[2:]
	n, err := getOrSetDeleteDeadline.Run(ctx, s.client, []string{rk.hashKey},
		"deadline:"+suffix, "ts:"+suffix, proposed, time.Now().Unix()).Int64()
	if err == redis.Nil {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(n), nil
}

func (s *versionSvc) GetACL(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) (Version, error) {
	return s.getObjVersion(ctx, replID, obj, acl)
}

func (s *versionSvc) IncrementACL(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination) (int, error) {
	keys, err := toRedisObjKeys(replID, obj, acl)
	if err != nil {
		return 0, err
	}
	return s.incrementDest(ctx, replID, keys, dest)
}

func (s *versionSvc) UpdateACLIfGreater(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination, version int) error {
	keys, err := toRedisObjKeys(replID, obj, acl)
	if err != nil {
		return err
	}
	return s.setDestIfGreater(ctx, replID, keys, dest, version)
}

func (s *versionSvc) GetTags(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object) (Version, error) {
	return s.getObjVersion(ctx, replID, obj, tags)
}

func (s *versionSvc) IncrementTags(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination) (int, error) {
	keys, err := toRedisObjKeys(replID, obj, tags)
	if err != nil {
		return 0, err
	}
	return s.incrementDest(ctx, replID, keys, dest)
}

func (s *versionSvc) UpdateTagsIfGreater(ctx context.Context, replID entity.UniversalReplicationID, obj dom.Object, dest Destination, version int) error {
	keys, err := toRedisObjKeys(replID, obj, tags)
	if err != nil {
		return err
	}
	return s.setDestIfGreater(ctx, replID, keys, dest, version)
}

func (s *versionSvc) GetBucket(ctx context.Context, replID entity.UniversalReplicationID, bucket string) (Version, error) {
	return s.getBucketVersion(ctx, replID, bucket, payload)
}

func (s *versionSvc) IncrementBucket(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination) (int, error) {
	keys, err := toRedisBucketKeys(replID, bucket, payload)
	if err != nil {
		return 0, err
	}
	return s.incrementDest(ctx, replID, keys, dest)
}

func (s *versionSvc) UpdateBucketIfGreater(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination, version int) error {
	keys, err := toRedisBucketKeys(replID, bucket, payload)
	if err != nil {
		return err
	}
	return s.setDestIfGreater(ctx, replID, keys, dest, version)
}

func (s *versionSvc) DeleteBucketAll(ctx context.Context, replID entity.UniversalReplicationID, bucket string) error {
	key, fields, err := allRedisBucketKeys(replID, bucket)
	if err != nil {
		return err
	}
	return s.client.HDel(ctx, key, fields...).Err()
}

func (s *versionSvc) GetBucketACL(ctx context.Context, replID entity.UniversalReplicationID, bucket string) (Version, error) {
	return s.getBucketVersion(ctx, replID, bucket, acl)
}

func (s *versionSvc) IncrementBucketACL(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination) (int, error) {
	keys, err := toRedisBucketKeys(replID, bucket, acl)
	if err != nil {
		return 0, err
	}
	return s.incrementDest(ctx, replID, keys, dest)
}

func (s *versionSvc) UpdateBucketACLIfGreater(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination, version int) error {
	keys, err := toRedisBucketKeys(replID, bucket, acl)
	if err != nil {
		return err
	}
	return s.setDestIfGreater(ctx, replID, keys, dest, version)
}

func (s *versionSvc) GetBucketTags(ctx context.Context, replID entity.UniversalReplicationID, bucket string) (Version, error) {
	return s.getBucketVersion(ctx, replID, bucket, tags)
}

func (s *versionSvc) IncrementBucketTags(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination) (int, error) {
	keys, err := toRedisBucketKeys(replID, bucket, tags)
	if err != nil {
		return 0, err
	}
	return s.incrementDest(ctx, replID, keys, dest)
}

func (s *versionSvc) UpdateBucketTagsIfGreater(ctx context.Context, replID entity.UniversalReplicationID, bucket string, dest Destination, version int) error {
	keys, err := toRedisBucketKeys(replID, bucket, tags)
	if err != nil {
		return err
	}
	return s.setDestIfGreater(ctx, replID, keys, dest, version)
}

func (s *versionSvc) Cleanup(ctx context.Context, replID entity.UniversalReplicationID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pattern := redisReplicationMATCH(replID)
	if !strings.HasSuffix(pattern, "*") {
		// exact match - single key deletion
		return s.client.Unlink(ctx, pattern).Err()
	}
	// The API holds the replication's mutation lock and stops its queues before
	// cleanup. Scan from the client so other Redis requests run between batches.
	const batchSize = 1000
	var cursor uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		keys, next, err := s.client.Scan(ctx, cursor, pattern, batchSize).Result()
		if err != nil {
			return err
		}
		// SCAN's COUNT is a hint, so bound UNLINK independently as well.
		for start := 0; start < len(keys); start += batchSize {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := s.client.Unlink(ctx, keys[start:min(start+batchSize, len(keys))]...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (s *versionSvc) getObjVersion(ctx context.Context, replID entity.UniversalReplicationID, object dom.Object, kind versionKind) (Version, error) {
	keys, err := toRedisObjKeys(replID, object, kind)
	if err != nil {
		return Version{}, err
	}
	return s.getVersion(ctx, keys)
}

func (s *versionSvc) getBucketVersion(ctx context.Context, replID entity.UniversalReplicationID, bucket string, kind versionKind) (Version, error) {
	keys, err := toRedisBucketKeys(replID, bucket, kind)
	if err != nil {
		return Version{}, err
	}
	return s.getVersion(ctx, keys)
}

func (s *versionSvc) getVersion(ctx context.Context, keys redisKeys) (Version, error) {
	res, err := s.client.HMGet(ctx, keys.hashKey, keys.fromField, keys.toField).Result()
	if err != nil {
		return Version{}, err
	}
	if len(res) != 2 {
		return Version{}, fmt.Errorf("%w: unexpected HMGet result length %d", dom.ErrInternal, len(res))
	}
	var version Version
	if res[0] != nil {
		version.From, err = strconv.Atoi(res[0].(string))
		if err != nil {
			return Version{}, fmt.Errorf("%w: unable to parse from version %q", dom.ErrInternal, res[0])
		}
	}
	if res[1] != nil {
		version.To, err = strconv.Atoi(res[1].(string))
		if err != nil {
			return Version{}, fmt.Errorf("%w: unable to parse to version %q", dom.ErrInternal, res[1])
		}
	}
	return version, nil
}

func (s *versionSvc) incrementDest(ctx context.Context, replID entity.UniversalReplicationID, keys redisKeys, dest Destination) (int, error) {
	if err := dest.Validate(); err != nil {
		return 0, err
	}
	isFrom, err := dest.IsFrom(replID)
	if err != nil {
		return 0, err
	}
	if isFrom {
		return s.incVersion(ctx, keys.hashKey, keys.fromField, keys.toField)
	}
	// swap fields for 'to' destination
	return s.incVersion(ctx, keys.hashKey, keys.toField, keys.fromField)
}

// luaHIncVersion - takes max value out of hash fields (A,B,sequence), increments it, and assigns result to A and sequence.
//
//	A = max(A, B, sequence) + 1
//	sequence = A
//
// Input:
// KEYS[1] -> Redis hash Key
// --
// ARGV[1] -> First hash field (A)
// ARGV[2] -> Second hash field (B)
//
// Output:
// Returns incremented version
var luaHIncVersion = redis.NewScript(`
local max_ver = 0
local sequence = 's:' .. string.sub(ARGV[1], 3)
local vals = redis.call('HMGET', KEYS[1], ARGV[1], ARGV[2], sequence)
for _, ver in ipairs(vals) do
	if ver then
		max_ver = math.max(max_ver, tonumber(ver))
	end
end
max_ver = max_ver + 1
redis.call('HSET', KEYS[1], ARGV[1], max_ver, sequence, max_ver)
local ts = 'ts:' .. string.sub(ARGV[1], 3)
redis.call('HDEL', KEYS[1], ts)
if ARGV[3] == 'source-object' then redis.call('HDEL', KEYS[1], 'deadline:' .. string.sub(ARGV[1], 3)) end
return max_ver`)

func (s *versionSvc) incObjectSourceVersion(ctx context.Context, keys redisKeys) (int, error) {
	result, err := luaHIncVersion.Run(ctx, s.client, []string{keys.hashKey}, keys.fromField, keys.toField, "source-object").Int64()
	if err != nil {
		return 0, err
	}
	return int(result), nil
}

func (s *versionSvc) incVersion(ctx context.Context, key, a, b string) (int, error) {
	result, err := luaHIncVersion.Run(ctx, s.client, []string{key}, a, b).Result()
	if err != nil {
		return 0, err
	}
	inc, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("%w: unable to cast luaHIncVersion result %T to int64", dom.ErrInternal, result)
	}
	if inc == 0 {
		return 0, dom.ErrNotFound
	}
	return int(inc), nil
}

func (s *versionSvc) setDestIfGreater(ctx context.Context, replID entity.UniversalReplicationID, keys redisKeys, dest Destination, version int) error {
	if err := dest.Validate(); err != nil {
		return err
	}
	isFrom, err := dest.IsFrom(replID)
	if err != nil {
		return err
	}
	if isFrom {
		return s.setIfGreater(ctx, keys.hashKey, keys.fromField, keys.toField, version)
	}
	// swap fields for 'to' destination
	return s.setIfGreater(ctx, keys.hashKey, keys.toField, keys.fromField, version)
}

// luaHSetIfExAndGreater(hash_key, field_A, field_B, X) - sets X to hash field A only if:
//  1. hash key exists
//  2. field B exists   - checks concurrent delete
//  3. X <= B           - checks concurrent delete
//  4. X > A            - checks concurrent updates
//
// Input:
// KEYS[1] -> Redis hash Key
// --
// ARGV[1] -> First Hash field (A)
// ARGV[2] -> Second Hash field (B)
// ARGV[3] -> Integer value (X) to set to first hash field
//
// Output:
// Returns -1 if hash key does not exist or second field does not exist or less than given integer
// Returns 0 if given integer is not greater than current value
// Returns 1 if value was set
var luaHSetIfExAndGreater = redis.NewScript(`
if redis.call('exists',KEYS[1]) ~= 1 then return -1 end

local other = redis.call('hget', KEYS[1], ARGV[2])
if not other or tonumber(other) < tonumber(ARGV[3]) then return -1 end

local prev = redis.call('hget', KEYS[1], ARGV[1])
if prev and (tonumber(prev) >= tonumber(ARGV[3])) then return 0
else 
	local sequence = 's:' .. string.sub(ARGV[1], 3)
	local seq_val = redis.call('hget', KEYS[1], sequence)
	local max_seq = tonumber(ARGV[3])
	if seq_val and tonumber(seq_val) > max_seq then
		max_seq = tonumber(seq_val)
	end
	redis.call("hset", KEYS[1], ARGV[1], ARGV[3], sequence, max_seq)
	return 1
end`)

func (s *versionSvc) setIfGreater(ctx context.Context, key, a, b string, version int) error {
	if version <= 0 {
		zerolog.Ctx(ctx).Warn().Str("key", key).Str("field", a).Msgf("discarded attempt to set invalid version %d", version)
		return fmt.Errorf("%w: version value must be positive", dom.ErrInvalidArg)
	}
	result, err := luaHSetIfExAndGreater.Run(ctx, s.client, []string{key}, a, b, version).Result()
	if err != nil {
		return err
	}
	inc, ok := result.(int64)
	if !ok {
		return fmt.Errorf("%w: unable to cast luaHSetIfExAndGreater result %T to int64", dom.ErrInternal, result)
	}
	if inc == -1 {
		return fmt.Errorf("%w: unable to update replicated version for key %s", dom.ErrNotFound, key)
	}
	if inc == 0 {
		return fmt.Errorf("%w: unable to update replicated version for key %s", dom.ErrAlreadyExists, key)
	}
	return nil
}

var pruneObjectTombstones = redis.NewScript(`
local count = 0
for i = 2, #ARGV, 2 do
	local timestampField = ARGV[i]
	local suffix = string.sub(timestampField, 4)
	local deadlineField = 'deadline:' .. suffix
	local deadline = redis.call('HGET', KEYS[1], deadlineField)
	if redis.call('HGET', KEYS[1], timestampField) == ARGV[i+1]
		and (not deadline or tonumber(deadline) <= tonumber(ARGV[1])) then
		redis.call('HDEL', KEYS[1], timestampField, 's:' .. suffix, deadlineField)
		count = count + 1
	end
end
return count`)

// GCObjectTombstones prunes version metadata after the retention window has
// elapsed since both the deletion and its propagation deadline.
func (s *versionSvc) GCObjectTombstones(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	cutoff := time.Now().Add(-maxAge)

	var totalPruned int
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, "v:*", 100).Result()
		if err != nil {
			return totalPruned, err
		}
		for _, key := range keys {
			var hcursor uint64
			for {
				fields, nextHCursor, err := s.client.HScan(ctx, key, hcursor, "ts:*", 100).Result()
				if err != nil {
					break
				}
				candidates := []any{cutoff.UnixMilli()}
				for i := 0; i < len(fields); i += 2 {
					tsVal, err := strconv.ParseInt(fields[i+1], 10, 64)
					if err == nil && tsVal <= cutoff.Unix() {
						candidates = append(candidates, fields[i], fields[i+1])
					}
				}
				if len(candidates) > 1 {
					n, err := pruneObjectTombstones.Run(ctx, s.client, []string{key}, candidates...).Int()
					if err != nil {
						return totalPruned, err
					}
					totalPruned += n
				}
				hcursor = nextHCursor
				if hcursor == 0 {
					break
				}
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return totalPruned, nil
}

// RunTombstoneGC periodically purges expired object sequence tombstones until ctx is canceled.
func RunTombstoneGC(ctx context.Context, svc VersionService, interval time.Duration, maxAge time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := svc.GCObjectTombstones(ctx, maxAge); err != nil && onError != nil && ctx.Err() == nil {
				onError(err)
			}
		}
	}
}
