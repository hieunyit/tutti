package meta

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func TestVersionCleanupPreservesOtherReplications(t *testing.T) {
	c := testutil.SetupRedis(t)
	ctx := t.Context()
	id := entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("user", "source", "target"))
	preserved := []string{"v:other:source:target:b:b", "v:user:source:other:b:b", "v:user:other:target:b:b", "other:user:source:target:b:b"}
	pipe := c.Pipeline()
	for i := range 2105 {
		pipe.HSet(ctx, fmt.Sprintf("v:user:source:target:b%d:b%d", i, i), "version", 1)
	}
	for _, key := range preserved {
		pipe.HSet(ctx, key, "version", 1)
	}
	_, err := pipe.Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, NewVersionService(c).Cleanup(ctx, id))
	keys, err := c.Keys(ctx, "*").Result()
	require.NoError(t, err)
	require.ElementsMatch(t, preserved, keys)
}

// Override only the commands Cleanup uses so cursor, failure, and oversized
// COUNT-hint responses can be exercised without relying on server heuristics.
type cleanupClient struct {
	redis.UniversalClient
	scan   func(context.Context, uint64, string, int64) *redis.ScanCmd
	unlink func(context.Context, ...string) *redis.IntCmd
}

func (c cleanupClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return c.scan(ctx, cursor, match, count)
}

func (c cleanupClient) Unlink(ctx context.Context, keys ...string) *redis.IntCmd {
	return c.unlink(ctx, keys...)
}

func TestVersionCleanupBoundsUnlinkAndContinuesEmptyPages(t *testing.T) {
	id := entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("user", "source", "target"))
	keys := make([]string, 2501)
	for i := range keys {
		keys[i] = fmt.Sprintf("v:user:source:target:b%d:b%d", i, i)
	}
	var cursors []uint64
	var batches []int
	var deleted []string
	c := cleanupClient{
		scan: func(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
			require.Equal(t, "v:user:source:target:*", match)
			require.EqualValues(t, 1000, count)
			cursors = append(cursors, cursor)
			cmd := redis.NewScanCmd(ctx, nil)
			if cursor == 0 {
				cmd.SetVal(nil, 7) // Empty pages do not mean iteration is complete.
			} else {
				cmd.SetVal(keys, 0) // COUNT can return more than the requested hint.
			}
			return cmd
		},
		unlink: func(ctx context.Context, batch ...string) *redis.IntCmd {
			batches = append(batches, len(batch))
			deleted = append(deleted, batch...)
			return redis.NewIntResult(int64(len(batch)), nil)
		},
	}
	require.NoError(t, NewVersionService(c).Cleanup(t.Context(), id))
	require.Equal(t, []uint64{0, 7}, cursors)
	require.Equal(t, []int{1000, 1000, 501}, batches)
	require.Equal(t, keys, deleted)
}

func TestVersionCleanupStopsOnFailureOrCancellation(t *testing.T) {
	userID := entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("user", "source", "target"))
	bucketID := entity.UniversalFromBucketReplication(entity.NewBucketReplicationPolicy("user", "source", "bucket", "target", "bucket"))
	injected := errors.New("redis unavailable")
	for _, mode := range []string{"scan error", "unlink error", "cancel between batches", "cancel before scan", "cancel before exact unlink", "exact unlink error"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			wantErr := injected
			id := userID
			if mode == "cancel before exact unlink" || mode == "exact unlink error" {
				id = bucketID
			}
			if mode == "cancel before scan" || mode == "cancel before exact unlink" {
				cancel()
				wantErr = context.Canceled
			}
			if mode == "cancel between batches" {
				wantErr = context.Canceled
			}
			scans, unlinks := 0, 0
			c := cleanupClient{
				scan: func(ctx context.Context, _ uint64, _ string, _ int64) *redis.ScanCmd {
					scans++
					cmd := redis.NewScanCmd(ctx, nil)
					if mode == "scan error" {
						cmd.SetErr(injected)
					} else {
						cmd.SetVal(make([]string, 1001), 7)
					}
					return cmd
				},
				unlink: func(_ context.Context, keys ...string) *redis.IntCmd {
					unlinks++
					if mode == "exact unlink error" {
						require.Equal(t, []string{"v:user:source:target:bucket:bucket"}, keys)
					}
					if mode == "cancel between batches" {
						cancel()
						return redis.NewIntResult(int64(len(keys)), nil)
					}
					return redis.NewIntResult(0, injected)
				},
			}
			require.ErrorIs(t, NewVersionService(c).Cleanup(ctx, id), wantErr)
			if mode == "cancel before scan" || mode == "cancel before exact unlink" {
				if scans != 0 || unlinks != 0 {
					t.Fatal("canceled cleanup issued Redis commands")
				}
			} else if mode == "scan error" {
				require.Equal(t, 1, scans)
				require.Zero(t, unlinks)
			} else {
				require.Equal(t, 1, unlinks, "failure must stop before the next delete batch")
			}
		})
	}
}
