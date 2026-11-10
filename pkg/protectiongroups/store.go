// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"time"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Store struct {
	Redis redis.UniversalClient
	CP    controlplane.Store
}

func NewStore(r redis.UniversalClient) *Store { return &Store{Redis: r} }

// WithControlPlane attaches a PostgreSQL control-plane store as the source of truth for desired group configuration
// (GroupSpec), while runtime promotion and fence state (PromotionState) remains in Redis.
func (s *Store) WithControlPlane(cp controlplane.Store) *Store {
	s.CP = cp
	return s
}

func (s *Store) Get(ctx context.Context, id string) (Group, error) {
	if s.CP != nil {
		doc, err := s.CP.Repos().ProtectionGroups.Get(ctx, id)
		if errors.Is(err, dom.ErrNotFound) {
			return Group{}, dom.ErrNotFound
		}
		if err == nil {
			var pgGroup Group
			if uerr := json.Unmarshal(doc.Value.JSON, &pgGroup); uerr != nil {
				return Group{}, uerr
			}
			return s.mergeWithRedisPromotion(ctx, pgGroup), nil
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("group", id).Msg("protectiongroups: PostgreSQL read failed, falling back to Redis")
	}
	return s.getRedis(ctx, id)
}

func (s *Store) getRedis(ctx context.Context, id string) (Group, error) {
	raw, err := s.Redis.HGet(ctx, GroupsKey, id).Result()
	if errors.Is(err, redis.Nil) {
		return Group{}, dom.ErrNotFound
	}
	if err != nil {
		return Group{}, err
	}
	var g Group
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return Group{}, err
	}
	if pRaw, err := s.Redis.HGet(ctx, PromotionKey, id).Result(); err == nil {
		var p PromotionState
		if json.Unmarshal([]byte(pRaw), &p) == nil && p.Revision >= g.Revision {
			g.ApplyPromotion(p)
		}
	}
	return g, nil
}

func (s *Store) mergeWithRedisPromotion(ctx context.Context, pgGroup Group) Group {
	rGroup, err := s.getRedis(ctx, pgGroup.ID)
	if err != nil {
		// Redis lost or missing this group: heal Redis from the authoritative PostgreSQL record.
		healed := MergeGroup(pgGroup.Spec(), pgGroup.Promotion())
		if healed.Revision == 0 {
			healed.Revision = 1
		}
		_ = s.healRedis(ctx, healed)
		return healed
	}
	prom := rGroup.Promotion()
	merged := MergeGroup(pgGroup.Spec(), prom)
	if !rGroup.Spec().Equal(pgGroup.Spec()) {
		_ = s.healRedis(ctx, merged)
	}
	return merged
}

func (s *Store) healRedis(ctx context.Context, g Group) error {
	raw, err := json.Marshal(g)
	if err != nil {
		return err
	}
	promRaw, err := json.Marshal(g.Promotion())
	if err != nil {
		return err
	}
	docs, err := s.Redis.HGetAll(ctx, GroupsKey).Result()
	if err != nil {
		return err
	}
	index, err := scopeIndex(docs, &g, "")
	if err != nil {
		return err
	}
	currentIndex, err := s.Redis.HGetAll(ctx, ScopeIndexKey).Result()
	if err != nil {
		return err
	}
	_, err = s.Redis.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, GroupsKey, g.ID, raw)
		p.HSet(ctx, PromotionKey, g.ID, promRaw)
		writeIndex(ctx, p, currentIndex, index)
		return nil
	})
	return err
}

func (s *Store) List(ctx context.Context) ([]Group, error) {
	if s.CP != nil {
		docs, err := s.CP.Repos().ProtectionGroups.List(ctx)
		if err == nil {
			out := make([]Group, 0, len(docs))
			for _, d := range docs {
				var pgGroup Group
				if uerr := json.Unmarshal(d.Value.JSON, &pgGroup); uerr != nil {
					return nil, uerr
				}
				out = append(out, s.mergeWithRedisPromotion(ctx, pgGroup))
			}
			sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
			return out, nil
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("protectiongroups: PostgreSQL list failed, falling back to Redis")
	}
	raw, err := s.Redis.HGetAll(ctx, GroupsKey).Result()
	if err != nil {
		return nil, err
	}
	proms, _ := s.Redis.HGetAll(ctx, PromotionKey).Result()
	out := make([]Group, 0, len(raw))
	for id, v := range raw {
		var g Group
		if err := json.Unmarshal([]byte(v), &g); err != nil {
			return nil, err
		}
		if pRaw, ok := proms[id]; ok {
			var p PromotionState
			if json.Unmarshal([]byte(pRaw), &p) == nil && p.Revision >= g.Revision {
				g.ApplyPromotion(p)
			}
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Save performs compare-and-swap and overlap validation in one Redis transaction.
// When PostgreSQL control plane is enabled, desired configuration changes (create, spec update, or primary commit)
// are persisted to PostgreSQL as the source of truth before updating the Redis promotion/fence projection.
func (s *Store) Save(ctx context.Context, g *Group) error {
	expected := g.Revision
	var nextDoc []byte
	var syncPG bool
	err := s.Redis.Watch(ctx, func(tx *redis.Tx) error {
		docs, err := tx.HGetAll(ctx, GroupsKey).Result()
		if err != nil {
			return err
		}
		var current Group
		_, existed := docs[g.ID]
		if raw, ok := docs[g.ID]; ok {
			if err = json.Unmarshal([]byte(raw), &current); err != nil {
				return err
			}
		}
		if current.Revision != expected {
			return fmt.Errorf("%w: group changed; reload before retrying", dom.ErrFailedPrecondition)
		}
		for id, raw := range docs {
			if id == g.ID {
				continue
			}
			var other Group
			if err = json.Unmarshal([]byte(raw), &other); err != nil {
				return err
			}
			if g.Overlaps(other) {
				return fmt.Errorf("%w: scope overlaps group %s", dom.ErrAlreadyExists, id)
			}
		}
		next := *g
		next.Revision = expected + 1
		raw, err := json.Marshal(next)
		if err != nil {
			return err
		}
		promRaw, err := json.Marshal(next.Promotion())
		if err != nil {
			return err
		}
		nextDoc = raw
		syncPG = !existed || !current.Spec().Equal(next.Spec()) || current.Primary != next.Primary || current.Generation != next.Generation
		if s.CP != nil && syncPG {
			if err := s.saveControlPlane(ctx, next, raw, !existed); err != nil {
				return err
			}
		}
		index, err := scopeIndex(docs, &next, "")
		if err != nil {
			return err
		}
		currentIndex, err := tx.HGetAll(ctx, ScopeIndexKey).Result()
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.HSet(ctx, GroupsKey, g.ID, raw)
			p.HSet(ctx, PromotionKey, g.ID, promRaw)
			writeIndex(ctx, p, currentIndex, index)
			return nil
		})
		return err
	}, GroupsKey, ScopeIndexKey)
	if err == nil {
		g.Revision++
	}
	_ = nextDoc
	return err
}

func (s *Store) saveControlPlane(ctx context.Context, next Group, raw []byte, isCreate bool) error {
	pgDocs, err := s.CP.Repos().ProtectionGroups.List(ctx)
	if err != nil {
		return wrapCPError(err)
	}
	for _, d := range pgDocs {
		if d.Value.ID == next.ID {
			continue
		}
		var other Group
		if json.Unmarshal(d.Value.JSON, &other) == nil && next.Overlaps(other) {
			return fmt.Errorf("%w: scope overlaps group %s", dom.ErrAlreadyExists, d.Value.ID)
		}
	}
	doc := controlplane.RawDoc{ID: next.ID, User: next.User, JSON: raw}
	if isCreate {
		_, err = s.CP.Repos().ProtectionGroups.Create(ctx, doc)
	} else {
		_, err = s.CP.Repos().ProtectionGroups.Put(ctx, doc)
	}
	return wrapCPError(err)
}

func wrapCPError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, controlplane.ErrReadOnly) ||
		errors.Is(err, dom.ErrNotFound) ||
		errors.Is(err, dom.ErrAlreadyExists) ||
		errors.Is(err, dom.ErrInvalidArg) ||
		errors.Is(err, dom.ErrFailedPrecondition) {
		return err
	}
	return fmt.Errorf("%w: control plane unavailable: %v", dom.ErrUnavailable, err)
}

func (s *Store) Delete(ctx context.Context, g Group) error {
	return s.Redis.Watch(ctx, func(tx *redis.Tx) error {
		raw, err := tx.HGet(ctx, GroupsKey, g.ID).Result()
		if err != nil {
			return err
		}
		var current Group
		if err = json.Unmarshal([]byte(raw), &current); err != nil {
			return err
		}
		if current.Revision != g.Revision || current.State != "ready" {
			return fmt.Errorf("%w: only an unchanged ready group can be detached", dom.ErrFailedPrecondition)
		}
		if s.CP != nil {
			if err := s.CP.Repos().ProtectionGroups.Delete(ctx, g.ID); err != nil && !errors.Is(err, dom.ErrNotFound) {
				return wrapCPError(err)
			}
		}
		docs, err := tx.HGetAll(ctx, GroupsKey).Result()
		if err != nil {
			return err
		}
		index, err := scopeIndex(docs, nil, g.ID)
		if err != nil {
			return err
		}
		currentIndex, err := tx.HGetAll(ctx, ScopeIndexKey).Result()
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.HDel(ctx, GroupsKey, g.ID)
			p.HDel(ctx, PromotionKey, g.ID)
			writeIndex(ctx, p, currentIndex, index)
			return nil
		})
		return err
	}, GroupsKey, ScopeIndexKey)
}

// scopeIndex derives the complete admission index from the authoritative documents, replacing
// (or adding) next and dropping the group "without". Rebuilding the whole index on every
// create/delete keeps it exact even if it was lost or written by an older version.
func scopeIndex(docs map[string]string, next *Group, without string) (map[string]string, error) {
	index := map[string]string{indexVersionField: "1"}
	add := func(g Group) {
		if len(g.Buckets) == 0 {
			index["u:"+g.User] = g.ID
			return
		}
		for _, b := range g.Buckets {
			index["b:"+g.User+"/"+b] = g.ID
		}
	}
	for id, raw := range docs {
		if id == without || (next != nil && id == next.ID) {
			continue
		}
		var g Group
		if err := json.Unmarshal([]byte(raw), &g); err != nil {
			return nil, err
		}
		add(g)
	}
	if next != nil {
		add(*next)
	}
	return index, nil
}

// ConfigVersionKey and ConfigChangedChannel are the proxy policy snapshot's version counter and change channel
// (policy.SnapshotVersionKey / policy.SnapshotChannel; this package cannot import policy).
const (
	ConfigVersionKey     = "tutti:cfg:version"
	ConfigChangedChannel = "tutti:cfg:changed"
)

// writeIndex replaces the index when it differs from current and, in the same MULTI/EXEC, bumps the config
// version and notifies the proxies: their snapshots cache the index to admit unprotected writes from memory,
// and every write validates the version first, so a write after this commit always sees the new index.
func writeIndex(ctx context.Context, p redis.Pipeliner, current, index map[string]string) {
	if maps.Equal(current, index) {
		return
	}
	p.Del(ctx, ScopeIndexKey)
	p.HSet(ctx, ScopeIndexKey, index)
	p.Incr(ctx, ConfigVersionKey)
	p.Publish(ctx, ConfigChangedChannel, "changed")
}

// LookupScope resolves a (user, bucket) against a cached copy of the scope index. ok is false when the copy
// is not a complete index (marker missing): the caller must then use the Redis admission path.
func LookupScope(index map[string]string, user, bucket string) (group string, ok bool) {
	if index[indexVersionField] == "" {
		return "", false
	}
	if id := index["u:"+user]; id != "" {
		return id, true
	}
	return index["b:"+user+"/"+bucket], true
}

// EnsureIndex rebuilds the scope index when it is missing (groups written by an older version or after Redis loss).
func (s *Store) EnsureIndex(ctx context.Context) error {
	if s.CP != nil {
		_, _ = s.List(ctx)
	}
	return s.Redis.Watch(ctx, func(tx *redis.Tx) error {
		ok, err := tx.HExists(ctx, ScopeIndexKey, indexVersionField).Result()
		if err != nil || ok {
			return err
		}
		docs, err := tx.HGetAll(ctx, GroupsKey).Result()
		if err != nil {
			return err
		}
		index, err := scopeIndex(docs, nil, "")
		if err != nil {
			return err
		}
		current, err := tx.HGetAll(ctx, ScopeIndexKey).Result()
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error { writeIndex(ctx, p, current, index); return nil })
		return err
	}, GroupsKey, ScopeIndexKey)
}

func (s *Store) ActiveWriters(ctx context.Context, id string) (int64, error) {
	return s.Redis.HLen(ctx, writerKey(id)).Result()
}
func writerKey(id string) string { return "tutti:protection:writers:" + id }
func (s *Store) AutomaticFrozen(ctx context.Context) (bool, error) {
	v, err := s.Redis.Get(ctx, FreezeKey).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return v == "1", err
}
func (s *Store) SetAutomaticFrozen(ctx context.Context, frozen bool) error {
	v := "0"
	if frozen {
		v = "1"
	}
	return s.Redis.Set(ctx, FreezeKey, v, 0).Err()
}

// References are durable even while a member is down or its replication is not yet created.
func (s *Store) References(ctx context.Context, user, storage string) ([]string, error) {
	groups, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, g := range groups {
		match := user != "" && g.User == user
		for _, m := range g.Members {
			match = match || (storage != "" && m == storage)
		}
		if match {
			out = append(out, "protection group "+g.ID)
		}
	}
	return out, nil
}

// reapScript deletes one writer record only if it is unchanged and its process lease is not held
// by the instance that admitted it. KEYS[1]=writers KEYS[2]=lease; ARGV: token, instance, record.
var reapScript = redis.NewScript(`
if redis.call('get',KEYS[2])==ARGV[2] then return 0 end
if redis.call('hget',KEYS[1],ARGV[1])~=ARGV[3] then return 0 end
return redis.call('hdel',KEYS[1],ARGV[1])
`)

// ReapWriters removes writer records of processes that are proven gone, so a crashed proxy or
// worker cannot block promotion forever. A record is reaped only when BOTH hold:
//   - its process lease (Lease.Run, refreshed every ttl/6) is expired or now held by a newer
//     instance of the same ID, and
//   - for proxies: the owner is absent from fresh proxy status reports, or its fresh report shows
//     a process started after the write was admitted (it restarted, the old instance is dead).
//
// A slow but alive proxy keeps refreshing its lease and is never reaped; a proxy partitioned
// from Redis self-fences (cancels its protected writes) after ttl/2 without a refresh, before
// its lease can expire. Workers do not publish status reports: their lease alone decides, and
// they self-fence the same way. Legacy records without a lease reference are never reaped.
func (s *Store) ReapWriters(ctx context.Context, id string, reports []proxystatus.Status, now time.Time, maxAge time.Duration) (int, error) {
	reaped := 0
	for _, kind := range []string{kindProxy, kindWorker} {
		key := writerKey(id)
		if kind == kindWorker {
			key += ":workers"
		}
		records, err := s.Redis.HGetAll(ctx, key).Result()
		if err != nil {
			return reaped, err
		}
		for token, value := range records {
			w, ok := parseWriter(value)
			if !ok {
				continue
			}
			if kind == kindProxy && proxyMayStillWrite(w, reports, now, maxAge) {
				continue
			}
			n, err := reapScript.Run(ctx, s.Redis, []string{key, LeaseKey(kind, w.owner)}, token, w.instance, value).Int()
			if err != nil {
				return reaped, err
			}
			if n > 0 {
				reaped++
				zerolog.Ctx(ctx).Warn().Str("group", id).Str("owner", w.owner).Str("kind", kind).Time("admitted", w.admitted).
					Msg("reaped writer record of a process whose lease expired and that is no longer reporting")
			}
		}
	}
	return reaped, nil
}

func proxyMayStillWrite(w writerRecord, reports []proxystatus.Status, now time.Time, maxAge time.Duration) bool {
	for _, r := range reports {
		if r.AppID != w.owner || !fresh(r, now, maxAge) {
			continue
		}
		// A fresh report of a process started after the admission proves a restart. Otherwise
		// the process that admitted the write may still be running.
		if r.StartedAt.IsZero() || !r.StartedAt.After(w.admitted) {
			return true
		}
	}
	return false
}
