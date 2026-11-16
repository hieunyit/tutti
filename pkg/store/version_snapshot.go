package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/redis/go-redis/v9"
)

// FindVersion locates a migration checkpoint without comparing listing-only
// timestamps, which cannot be recovered from the destination object's metadata.
// The caller holds the object lock, so the snapshot stays stable between pages.
func (s *ObjectVersionInfoStore) FindVersion(ctx context.Context, id entity.VersionedObjectID, checkpoint entity.ObjectVersionInfo) (int64, error) {
	const pageSize = 1000
	for start := uint64(0); ; start += pageSize {
		page, err := s.GetPage(ctx, id, NewPager(start, pageSize))
		if err != nil {
			return 0, err
		}
		for i, version := range page {
			if version.Version == checkpoint.Version && version.Size == checkpoint.Size && version.DeleteMarker == checkpoint.DeleteMarker {
				return int64(start) + int64(i), nil
			}
		}
		if len(page) < pageSize {
			return 0, dom.ErrNotFound
		}
	}
}

// ForReplication isolates work and cleanup between destinations and generations.
// Legacy source-only lists are deliberately not reused: a pending legacy copy
// task rebuilds its own snapshot from the source before proceeding.
func (s *ObjectVersionInfoStore) ForReplication(id entity.BucketReplicationPolicy, generation int64) *ObjectVersionInfoStore {
	uid := entity.UniversalFromBucketReplication(id)
	scope := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", uid.AsString(), generation)))
	return &ObjectVersionInfoStore{*NewRedisIDKeyList[entity.VersionedObjectID, entity.ObjectVersionInfo](
		s.Client(), fmt.Sprintf("p:repl:objectversion:v2:%x", scope),
		ObjectVersionIDToTokensConverter, TokensToObjectVersionIDConverter,
		ObjectVersionInfoToStringConverter, StringToObjectVersionInfoConverter)}
}

var replaceVersionSnapshot = redis.NewScript(`
redis.call("DEL", KEYS[1])
for i = 1, #ARGV do redis.call("RPUSH", KEYS[1], ARGV[i]) end
return #ARGV
`)

// ReplaceSnapshot installs a complete, chronologically ordered source listing.
// The caller holds the destination object lock, shared by listing and copying.
func (s *ObjectVersionInfoStore) ReplaceSnapshot(ctx context.Context, id entity.VersionedObjectID, versions []entity.ObjectVersionInfo) error {
	key, err := s.MakeKey(id)
	if err != nil {
		return err
	}
	args := make([]any, len(versions))
	for i, v := range versions {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		args[i] = string(raw)
	}
	return replaceVersionSnapshot.Run(ctx, s.Client(), []string{key}, args...).Err()
}
