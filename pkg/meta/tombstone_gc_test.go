package meta

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func TestGCObjectTombstones(t *testing.T) {
	r := require.New(t)
	c := testutil.SetupRedis(t)
	s := NewVersionService(c)
	ctx := context.Background()

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "alice",
		FromStorage: "s1",
		ToStorage:   "s2",
		FromBucket:  "b1",
		ToBucket:    "b2",
	})
	obj := dom.Object{Bucket: "b1", Name: "temp.txt"}

	// 1. Increment object
	dest := Destination{Storage: "s1", Bucket: "b1"}
	v1, err := s.IncrementObj(ctx, replID, obj, dest)
	r.NoError(err)
	r.Equal(1, v1)

	// 2. Delete object - records tombstone
	err = s.DeleteObjAll(ctx, replID, obj)
	r.NoError(err)

	// Verify fields in Redis hash: f: and t: deleted, but s: and ts: exist
	rk, err := toRedisObjKeys(replID, obj, payload)
	r.NoError(err)
	hashKey := rk.hashKey
	tsField := "ts:" + rk.fromField[2:]
	seqField := "s:" + rk.fromField[2:]

	hasTs, err := c.HExists(ctx, hashKey, tsField).Result()
	r.NoError(err)
	r.True(hasTs, "tombstone field ts: must exist after delete")

	hasSeq, err := c.HExists(ctx, hashKey, seqField).Result()
	r.NoError(err)
	r.True(hasSeq, "sequence field s: must exist after delete to prevent immediate ABA")

	// 3. GC with high maxAge (1 hour) should NOT prune
	pruned, err := s.GCObjectTombstones(ctx, time.Hour)
	r.NoError(err)
	r.Equal(0, pruned)

	// Wait 1.1s so unix timestamp changes by at least 1s
	time.Sleep(1100 * time.Millisecond)

	// 4. GC with maxAge of 1s should prune the tombstone
	pruned, err = s.GCObjectTombstones(ctx, time.Second)
	r.NoError(err)
	r.GreaterOrEqual(pruned, 1)

	hasTsAfter, _ := c.HExists(ctx, hashKey, tsField).Result()
	r.False(hasTsAfter, "tombstone field should be pruned")
	hasSeqAfter, _ := c.HExists(ctx, hashKey, seqField).Result()
	r.False(hasSeqAfter, "sequence field should be pruned after tombstone expiration")
}
