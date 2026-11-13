// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"math/rand"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
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
	// SharedAccount reports whether requests of user on storage use the storage's shared connection account
	// (credential mode "connection"; the same predicate as the proxy and the worker bucket scope). When set,
	// WorkerMiddleware refuses replication writes of other identities into buckets a group manages.
	SharedAccount func(storage, user string) bool
}

func NewStore(r redis.UniversalClient) *Store { return &Store{Redis: r} }

// DeletedKey records the IDs of deleted groups while a PostgreSQL control plane is attached, so a PostgreSQL row
// whose delete failed (or that a concurrent best-effort sync re-created) is removed on the next read instead of
// resurrecting the group in Redis. A tombstone is cleared when a group with the same ID is created again.
const DeletedKey = "tutti:protection:deleted"

// WithControlPlane attaches a PostgreSQL control-plane store that keeps a durable copy of every group.
//
// Ownership of the fields (see docs/tutti/control-plane-db.md):
//   - Redis (GroupsKey/PromotionKey) is authoritative for everything while it holds the group: the admission and
//     fencing Lua scripts read it, every mutation is a WATCH/CAS transaction on it, and it is written FIRST.
//   - PostgreSQL holds the desired spec (GroupSpec) plus a recovery seed (primary, generation, quarantined members,
//     history), written AFTER Redis committed. Its runtime fields are always stored recovery-safe (frozen,
//     "initializing", no seeded/verified marks), so restoring a PostgreSQL row can never unfreeze a group or claim
//     replicas are verified. A failed PostgreSQL write leaves Redis new / PostgreSQL old; every Get/List
//     re-pushes the Redis document when the PostgreSQL copy differs (reconcile on read), so it is retried
//     automatically.
//   - A group missing from Redis but present in PostgreSQL (Redis lost) is restored from the recovery seed under
//     WATCH, frozen and re-initializing: the reconciler re-seeds and re-verifies every member before unfreezing.
func (s *Store) WithControlPlane(cp controlplane.Store) *Store {
	s.CP = cp
	return s
}

// controlPlaneReadOnly reports whether the attached control plane rejects mutations. Spec mutations are then
// refused before Redis is touched; runtime transitions (promotion, freeze) continue and sync later.
func (s *Store) controlPlaneReadOnly() bool {
	if s.CP == nil {
		return false
	}
	ro, ok := s.CP.(interface{ IsReadOnly() bool })
	return ok && ro.IsReadOnly()
}

func (s *Store) Get(ctx context.Context, id string) (Group, error) {
	g, err := s.getRedis(ctx, id)
	if s.CP == nil {
		return g, err
	}
	switch {
	case err == nil:
		s.syncControlPlane(ctx, g, nil, false)
		return g, nil
	case errors.Is(err, dom.ErrNotFound):
		return s.restore(ctx, id)
	default:
		// Redis unreachable: the recovery-safe PostgreSQL copy (always frozen) can still be shown; any
		// mutation needs Redis and fails there.
		zerolog.Ctx(ctx).Warn().Err(err).Str("group", id).Msg("protectiongroups: Redis read failed, serving the PostgreSQL copy")
		doc, pgErr := s.CP.Repos().ProtectionGroups.Get(ctx, id)
		if pgErr != nil {
			return Group{}, err
		}
		pgGroup, uerr := decodeControlPlane(doc.Value.JSON)
		if uerr != nil {
			return Group{}, err
		}
		return pgGroup, nil
	}
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

// controlPlaneDoc is the PostgreSQL copy of g: its spec plus a recovery-safe runtime seed. Restoring it (or an
// outdated republish writing it to Redis) can never admit writes or skip re-verification.
func controlPlaneDoc(g Group) Group {
	out := MergeGroup(g.Spec(), PromotionState{
		Primary:     g.Primary,
		Generation:  g.Generation,
		Revision:    g.Revision,
		State:       "initializing",
		Frozen:      true,
		Quarantined: slices.Clone(g.Quarantined),
		History:     slices.Clone(g.History),
	})
	return out
}

func decodeControlPlane(raw []byte) (Group, error) {
	var g Group
	if err := json.Unmarshal(raw, &g); err != nil {
		return Group{}, err
	}
	// documents written by older versions may carry live runtime fields: never trust them
	return controlPlaneDoc(g), nil
}

// controlPlaneEqual compares the fields PostgreSQL owns (spec and recovery seed).
func controlPlaneEqual(a, b Group) bool {
	return a.Spec().Equal(b.Spec()) && a.Primary == b.Primary && a.Generation == b.Generation &&
		slices.Equal(a.Quarantined, b.Quarantined) && len(a.History) == len(b.History)
}

// syncControlPlane pushes the authoritative Redis document g to PostgreSQL when the PostgreSQL copy is missing
// or differs. pgCopy is the PostgreSQL copy when already loaded (known=true; nil = missing). Best effort: failures
// are logged and retried by the next read.
func (s *Store) syncControlPlane(ctx context.Context, g Group, pgCopy *Group, known bool) {
	if s.CP == nil || s.controlPlaneReadOnly() {
		return
	}
	if !known {
		doc, err := s.CP.Repos().ProtectionGroups.Get(ctx, g.ID)
		switch {
		case err == nil:
			cp, uerr := decodeControlPlane(doc.Value.JSON)
			if uerr == nil {
				pgCopy = &cp
			}
		case errors.Is(err, dom.ErrNotFound):
		default:
			zerolog.Ctx(ctx).Warn().Err(err).Str("group", g.ID).Msg("protectiongroups: PostgreSQL read failed; sync deferred")
			return
		}
	}
	if pgCopy != nil && controlPlaneEqual(*pgCopy, g) {
		return
	}
	if err := s.putControlPlane(ctx, g); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("group", g.ID).Msg("protectiongroups: PostgreSQL sync failed; retried on next read")
	}
}

// ControlPlaneDocument is the JSON stored in PostgreSQL for g (spec plus recovery-safe runtime seed).
func ControlPlaneDocument(g Group) ([]byte, error) { return json.Marshal(controlPlaneDoc(g)) }

func (s *Store) putControlPlane(ctx context.Context, g Group) error {
	raw, err := ControlPlaneDocument(g)
	if err != nil {
		return err
	}
	_, err = s.CP.Repos().ProtectionGroups.Put(ctx, controlplane.RawDoc{ID: g.ID, User: g.User, JSON: raw})
	return wrapCPError(err)
}

// deleteControlPlane removes the PostgreSQL row of a deleted (tombstoned) group. Best effort.
func (s *Store) deleteControlPlane(ctx context.Context, id string) {
	if s.controlPlaneReadOnly() {
		return
	}
	if err := s.CP.Repos().ProtectionGroups.Delete(ctx, id); err != nil && !errors.Is(err, dom.ErrNotFound) {
		zerolog.Ctx(ctx).Warn().Err(err).Str("group", id).Msg("protectiongroups: PostgreSQL delete failed; retried on next read")
	}
}

// restore handles a group missing from Redis: a tombstoned one stays deleted (its leftover PostgreSQL row is
// removed), otherwise the PostgreSQL copy, if any, is restored recovery-safe.
func (s *Store) restore(ctx context.Context, id string) (Group, error) {
	deleted, err := s.Redis.HExists(ctx, DeletedKey, id).Result()
	if err != nil {
		return Group{}, err
	}
	if deleted {
		s.deleteControlPlane(ctx, id)
		return Group{}, dom.ErrNotFound
	}
	doc, err := s.CP.Repos().ProtectionGroups.Get(ctx, id)
	if errors.Is(err, dom.ErrNotFound) {
		return Group{}, dom.ErrNotFound
	}
	if err != nil {
		return Group{}, wrapCPError(err)
	}
	pgGroup, err := decodeControlPlane(doc.Value.JSON)
	if err != nil {
		return Group{}, err
	}
	if err := s.healRedis(ctx, pgGroup); err != nil {
		return Group{}, err
	}
	return s.getRedis(ctx, id)
}

// healRedis restores a group that is missing from Redis from its recovery-safe PostgreSQL copy. It runs under
// WATCH and writes only when the group is still absent and not tombstoned, so it can never overwrite a newer
// Redis document or resurrect a deleted group.
func (s *Store) healRedis(ctx context.Context, g Group) error {
	g = controlPlaneDoc(g)
	if g.Revision == 0 {
		g.Revision = 1
	}
	raw, err := json.Marshal(g)
	if err != nil {
		return err
	}
	promRaw, err := json.Marshal(g.Promotion())
	if err != nil {
		return err
	}
	err = s.watch(ctx, func(tx *redis.Tx) error {
		docs, err := tx.HGetAll(ctx, GroupsKey).Result()
		if err != nil {
			return err
		}
		if _, ok := docs[g.ID]; ok {
			return nil // healed concurrently or never lost
		}
		deleted, err := tx.HExists(ctx, DeletedKey, g.ID).Result()
		if err != nil || deleted {
			return err
		}
		for id, other := range docs {
			var o Group
			if err := json.Unmarshal([]byte(other), &o); err != nil {
				return err
			}
			if g.Overlaps(o) {
				return fmt.Errorf("%w: restoring group %s: scope overlaps group %s", dom.ErrFailedPrecondition, g.ID, id)
			}
		}
		index, err := scopeIndex(docs, &g, "")
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
	}, GroupsKey, PromotionKey, ScopeIndexKey, DeletedKey)
	if err == nil {
		zerolog.Ctx(ctx).Warn().Str("group", g.ID).Str("primary", g.Primary).
			Msg("protectiongroups: restored group missing from Redis from PostgreSQL; frozen until re-initialized")
	}
	return err
}

func (s *Store) listRedis(ctx context.Context) ([]Group, error) {
	raw, err := s.Redis.HGetAll(ctx, GroupsKey).Result()
	if err != nil {
		return nil, err
	}
	proms, err := s.Redis.HGetAll(ctx, PromotionKey).Result()
	if err != nil {
		return nil, err
	}
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

// List returns the Redis groups. With a control plane it also reconciles PostgreSQL: missing or outdated copies
// are re-pushed, rows of deleted groups are removed and groups lost from Redis are restored (see WithControlPlane).
func (s *Store) List(ctx context.Context) ([]Group, error) {
	groups, err := s.listRedis(ctx)
	if s.CP == nil {
		return groups, err
	}
	docs, pgErr := s.CP.Repos().ProtectionGroups.List(ctx)
	if err != nil {
		if pgErr != nil {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("protectiongroups: Redis list failed, serving the PostgreSQL copies")
		out := make([]Group, 0, len(docs))
		for _, d := range docs {
			g, uerr := decodeControlPlane(d.Value.JSON)
			if uerr != nil {
				return nil, uerr
			}
			out = append(out, g)
		}
		return out, nil
	}
	if pgErr != nil {
		zerolog.Ctx(ctx).Warn().Err(pgErr).Msg("protectiongroups: PostgreSQL list failed; sync deferred")
		return groups, nil
	}
	pgCopies := make(map[string]Group, len(docs))
	for _, d := range docs {
		g, uerr := decodeControlPlane(d.Value.JSON)
		if uerr != nil {
			zerolog.Ctx(ctx).Warn().Err(uerr).Str("group", d.Value.ID).Msg("protectiongroups: corrupted PostgreSQL copy")
			continue
		}
		pgCopies[d.Value.ID] = g
	}
	inRedis := make(map[string]bool, len(groups))
	for _, g := range groups {
		inRedis[g.ID] = true
		if cp, ok := pgCopies[g.ID]; ok {
			s.syncControlPlane(ctx, g, &cp, true)
		} else {
			s.syncControlPlane(ctx, g, nil, true)
		}
	}
	restored := false
	for id := range pgCopies {
		if inRedis[id] {
			continue
		}
		if _, err := s.restore(ctx, id); err != nil && !errors.Is(err, dom.ErrNotFound) {
			zerolog.Ctx(ctx).Warn().Err(err).Str("group", id).Msg("protectiongroups: unable to restore group from PostgreSQL")
			continue
		}
		restored = true
	}
	if restored {
		return s.listRedis(ctx)
	}
	return groups, nil
}

// Recover restores every group missing from Redis from PostgreSQL (recovery-safe, frozen) and rebuilds the scope
// index. Use it after Redis was lost or republished instead of writing PostgreSQL documents to Redis directly.
func (s *Store) Recover(ctx context.Context) error {
	if _, err := s.List(ctx); err != nil {
		return err
	}
	return s.EnsureIndex(ctx)
}

// Save performs compare-and-swap and overlap validation in one Redis transaction, which is the commit point.
// With a PostgreSQL control plane the spec and recovery seed are copied to PostgreSQL afterwards (see
// WithControlPlane); a failed copy is logged and retried by the next read, never undone in Redis.
func (s *Store) Save(ctx context.Context, g *Group) error {
	expected := g.Revision
	var next Group
	var syncPG bool
	if s.CP != nil {
		if err := s.checkControlPlaneOverlap(ctx, *g); err != nil {
			return err
		}
	}
	err := s.watch(ctx, func(tx *redis.Tx) error {
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
		next = *g
		next.Revision = expected + 1
		raw, err := json.Marshal(next)
		if err != nil {
			return err
		}
		promRaw, err := json.Marshal(next.Promotion())
		if err != nil {
			return err
		}
		specChanged := !existed || !current.Spec().Equal(next.Spec())
		if specChanged && s.controlPlaneReadOnly() {
			return controlplane.ErrReadOnly
		}
		syncPG = specChanged || !controlPlaneEqual(current, next)
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
			if !existed && s.CP != nil {
				p.HDel(ctx, DeletedKey, g.ID)
			}
			writeIndex(ctx, p, currentIndex, index)
			return nil
		})
		return err
	}, GroupsKey, ScopeIndexKey)
	if err != nil {
		return err
	}
	g.Revision++
	if s.CP != nil && syncPG {
		s.syncControlPlane(ctx, next, nil, true)
	}
	return nil
}

// checkControlPlaneOverlap rejects a new group whose scope overlaps a group that only PostgreSQL still knows
// (lost from Redis and not yet restored).
func (s *Store) checkControlPlaneOverlap(ctx context.Context, next Group) error {
	exists, err := s.Redis.HExists(ctx, GroupsKey, next.ID).Result()
	if err != nil || exists {
		return err
	}
	pgDocs, err := s.CP.Repos().ProtectionGroups.List(ctx)
	if err != nil {
		return wrapCPError(err)
	}
	deleted, err := s.Redis.HGetAll(ctx, DeletedKey).Result()
	if err != nil {
		return err
	}
	for _, d := range pgDocs {
		if d.Value.ID == next.ID || deleted[d.Value.ID] != "" {
			continue
		}
		var other Group
		if json.Unmarshal(d.Value.JSON, &other) == nil && next.Overlaps(other) {
			return fmt.Errorf("%w: scope overlaps group %s", dom.ErrAlreadyExists, d.Value.ID)
		}
	}
	return nil
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

// Delete detaches an unchanged ready group. Redis is the commit point; with a control plane the group is
// tombstoned in the same transaction and its PostgreSQL row removed afterwards (retried by reads on failure).
func (s *Store) Delete(ctx context.Context, g Group) error {
	if s.controlPlaneReadOnly() {
		return controlplane.ErrReadOnly
	}
	err := s.watch(ctx, func(tx *redis.Tx) error {
		raw, err := tx.HGet(ctx, GroupsKey, g.ID).Result()
		if errors.Is(err, redis.Nil) {
			return dom.ErrNotFound
		}
		if err != nil {
			return err
		}
		var current Group
		if err = json.Unmarshal([]byte(raw), &current); err != nil {
			return err
		}
		if pRaw, err := tx.HGet(ctx, PromotionKey, g.ID).Result(); err == nil {
			var p PromotionState
			if json.Unmarshal([]byte(pRaw), &p) == nil && p.Revision >= current.Revision {
				current.ApplyPromotion(p)
			}
		}
		if current.Revision != g.Revision || current.State != "ready" {
			return fmt.Errorf("%w: only an unchanged ready group can be detached", dom.ErrFailedPrecondition)
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
			if s.CP != nil {
				p.HSet(ctx, DeletedKey, g.ID, strconv.FormatInt(time.Now().UnixMilli(), 10))
			}
			writeIndex(ctx, p, currentIndex, index)
			return nil
		})
		return err
	}, GroupsKey, PromotionKey, ScopeIndexKey)
	if err != nil {
		return err
	}
	if s.CP != nil {
		s.deleteControlPlane(ctx, g.ID)
	}
	return nil
}

// indexVersion is the format of the scope index. Version 2 added the storage fields ("s:" and "o:"); an index
// of another version is rebuilt by EnsureIndex.
const indexVersion = "2"

// conflictMarker is stored in a storage field claimed by more than one group (only possible with documents
// written before overlap validation covered it). Lookups refuse such a scope.
const conflictMarker = "!"

// scopeIndex derives the complete admission index from the authoritative documents, replacing
// (or adding) next and dropping the group "without". Rebuilding the whole index on every
// create/delete keeps it exact even if it was lost or written by an older version.
//
// Fields: "u:<user>" and "b:<user>/<bucket>" (scope of the group's own user), "s:<storage>/<bucket>" for every
// member of a bucket group (the physical buckets a group manages, used to refuse writes of OTHER identities on a
// shared connection account) and "o:<group>" (the group's user).
func scopeIndex(docs map[string]string, next *Group, without string) (map[string]string, error) {
	index := map[string]string{indexVersionField: indexVersion}
	add := func(g Group) {
		index["o:"+g.ID] = g.User
		index["p:"+g.ID] = g.Primary
		if len(g.Members) > 0 {
			index["m:"+g.ID] = strings.Join(g.Members, ",")
		}
		if len(g.Buckets) == 0 {
			if g.User != "" {
				index["u:"+g.User] = g.ID
			} else {
				for _, m := range g.Members {
					k := "st:" + m
					if cur, ok := index[k]; ok && cur != g.ID {
						index[k] = conflictMarker
						continue
					}
					index[k] = g.ID
				}
			}
			return
		}
		for _, b := range g.Buckets {
			if g.User != "" {
				index["b:"+g.User+"/"+b] = g.ID
			} else {
				index["b:"+b] = g.ID
			}
			for _, m := range g.Members {
				k := "s:" + m + "/" + b
				if cur, ok := index[k]; ok && cur != g.ID {
					index[k] = conflictMarker
					continue
				}
				index[k] = g.ID
			}
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

// IndexComplete reports whether a cached copy of the scope index is complete and of the current format.
func IndexComplete(index map[string]string) bool { return index[indexVersionField] == indexVersion }

// PrimaryOfGroup returns the primary storage of groupID from a cached scope index, or "".
func PrimaryOfGroup(index map[string]string, groupID string) string {
	if groupID == "" {
		return ""
	}
	return index["p:"+groupID]
}

// LookupScope resolves a (user, bucket) against a cached copy of the scope index. ok is false when the copy
// is not a complete index (marker missing): the caller must then use the Redis admission path.
func LookupScope(index map[string]string, user, bucket string) (group string, ok bool) {
	if index[indexVersionField] == "" {
		return "", false
	}
	if user != "" {
		if id := index["u:"+user]; id != "" {
			return id, true
		}
		if id := index["b:"+user+"/"+bucket]; id != "" {
			return id, true
		}
	}
	if bucket != "" {
		if id := index["b:"+bucket]; id != "" {
			return id, true
		}
	}
	return "", true
}

// LookupStorageGroup resolves a storage name to its storage-level protection group ID.
func LookupStorageGroup(index map[string]string, storage string) (group string, ok bool) {
	if index[indexVersionField] == "" {
		return "", false
	}
	if storage == "" {
		return "", true
	}
	id := index["st:"+storage]
	if id == conflictMarker {
		return "", false
	}
	return id, true
}

// LookupEffectiveScope resolves the protection group governing a write to (user, bucket) routed to storage.
// It checks user/bucket scope first, then storage-level scope for the routed storage.
// ok is false when the index is incomplete or when route is empty but storage groups exist,
// requiring callers to fall back to authoritative Redis admission.
func LookupEffectiveScope(index map[string]string, user, bucket, route string) (group string, ok bool) {
	if index[indexVersionField] == "" {
		return "", false
	}
	if group, ok := LookupScope(index, user, bucket); !ok || group != "" {
		return group, ok
	}
	if route != "" {
		return LookupStorageGroup(index, route)
	}
	// Route is empty: check if any storage-level protection group exists.
	// If one does, an empty route is ambiguous and cannot safely be declared unprotected.
	for k := range index {
		if strings.HasPrefix(k, "st:") {
			return "", false
		}
	}
	return "", true
}

// LookupSharedGroup resolves the protection group of ANOTHER identity that manages bucket on storage, for a user
// whose requests on storage use the storage's shared connection account (credential mode "connection"). It
// is an O(1) lookup of the storage fields of the index: only bucket groups that list storage as a member
// count; whole-user groups of other users never do (their bucket set is not known to the proxy). group is ""
// when nothing matches or the match is the user's own group (enforced by the normal fence). ok is false when
// the copy is not a complete index of the current format. A storage/bucket claimed by several groups is an error.
func LookupSharedGroup(index map[string]string, storage, user, bucket string) (group string, ok bool, err error) {
	if !IndexComplete(index) {
		return "", false, nil
	}
	if storage == "" || bucket == "" {
		return "", true, nil
	}
	id := index["s:"+storage+"/"+bucket]
	switch {
	case id == "":
		return "", true, nil
	case id == conflictMarker:
		return "", true, fmt.Errorf("%w: bucket %s on storage %s is claimed by several protection groups", dom.ErrFailedPrecondition, bucket, storage)
	case index["o:"+id] == "" || index["o:"+id] == user:
		// Infrastructure groups (no user owner) or the user's own group are not "another identity".
		return "", true, nil
	}
	return id, true, nil
}

// ScopeFields reads the scope index fields LookupScope and LookupStorageGroup need for (user, bucket, storages): a partial index for that lookup.
func ScopeFields(ctx context.Context, r redis.Cmdable, user, bucket string, storages ...string) (map[string]string, error) {
	keys := []string{indexVersionField}
	if user != "" {
		keys = append(keys, "u:"+user, "b:"+user+"/"+bucket)
	}
	if bucket != "" {
		keys = append(keys, "b:"+bucket)
	}
	for _, s := range storages {
		if s != "" {
			keys = append(keys, "st:"+s)
		}
	}
	vals, err := r.HMGet(ctx, ScopeIndexKey, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for i, k := range keys {
		if v, ok := vals[i].(string); ok {
			out[k] = v
		}
	}
	return out, nil
}

// SharedGroupFields reads the scope index fields LookupSharedGroup needs for (storage, bucket): the format
// marker, the storage field and the owner of the group it names. The result is a partial index for that
// lookup only.
func SharedGroupFields(ctx context.Context, r redis.Cmdable, storage, bucket string) (map[string]string, error) {
	sk := "s:" + storage + "/" + bucket
	vals, err := r.HMGet(ctx, ScopeIndexKey, indexVersionField, sk).Result()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for i, k := range []string{indexVersionField, sk} {
		if v, ok := vals[i].(string); ok {
			out[k] = v
		}
	}
	if id := out[sk]; id != "" && id != conflictMarker {
		owner, err := r.HGet(ctx, ScopeIndexKey, "o:"+id).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		out["o:"+id] = owner
	}
	return out, nil
}

// EnsureIndex rebuilds the scope index when it is missing or of an older format (groups written by an older
// version or after Redis loss).
func (s *Store) EnsureIndex(ctx context.Context) error {
	return s.watch(ctx, func(tx *redis.Tx) error {
		v, err := tx.HGet(ctx, ScopeIndexKey, indexVersionField).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		if v == indexVersion {
			return nil
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

// ActiveWritersBatch returns ActiveWriters of every id (same order) in one Redis round trip.
func (s *Store) ActiveWritersBatch(ctx context.Context, ids []string) ([]int64, error) {
	out := make([]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	cmds := make([]*redis.IntCmd, len(ids))
	_, err := s.Redis.Pipelined(ctx, func(p redis.Pipeliner) error {
		for i, id := range ids {
			cmds[i] = p.HLen(ctx, writerKey(id))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i, c := range cmds {
		out[i] = c.Val()
	}
	return out, nil
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
	if s.CP != nil {
		action := "ENABLE"
		if !frozen {
			action = "DISABLE"
		}
		if err := s.CP.Repos().Audit.Record(ctx, action, "protection_groups", "automatic_freeze", map[string]any{
			"frozen": frozen,
		}); err != nil {
			return fmt.Errorf("audit: unable to record automatic freeze intent: %w", err)
		}
	}
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

// watchAttempts bounds the optimistic-lock retries of watch.
const watchAttempts = 8

// watch runs a WATCH transaction and retries it, with jittered backoff, while another writer touched a watched key
// (redis.TxFailedErr). The closures only read watched state and rebuild their result on every run, so a retry is safe.
func (s *Store) watch(ctx context.Context, fn func(tx *redis.Tx) error, keys ...string) error {
	var err error
	for i := 0; i < watchAttempts; i++ {
		if err = s.Redis.Watch(ctx, fn, keys...); !errors.Is(err, redis.TxFailedErr) {
			return err
		}
		wait := time.Duration(5+rand.Intn(10*(i+1))) * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return err
}
