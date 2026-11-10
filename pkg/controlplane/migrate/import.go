// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

// Actor is the audit actor of imports.
const Actor = "tutti-migrate-pg"

// FamilyReport is the outcome for one family.
type FamilyReport struct {
	Source Summary `json:"source"`
	Target Summary `json:"target_before"`
	// After is the target summary after the import (equals Source on success; absent in dry-run).
	After     *Summary `json:"target_after,omitempty"`
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Unchanged int      `json:"unchanged"`
	// ExtraInTarget are records that exist only in PostgreSQL; they are kept (the import never deletes).
	ExtraInTarget int `json:"extra_in_target"`
	Errors        int `json:"errors"`
}

// Report is the verification report of an import or verify run.
type Report struct {
	StartedAt  time.Time                `json:"started_at"`
	DryRun     bool                     `json:"dry_run"`
	Families   map[Family]*FamilyReport `json:"families"`
	Warnings   []string                 `json:"warnings,omitempty"`
	Errors     []string                 `json:"errors,omitempty"`
	Mismatches []Diff                   `json:"mismatches,omitempty"`
	// OK is true if no error occurred and, after the import, source and target agree.
	OK bool `json:"ok"`
}

// ImportOptions of Import.
type ImportOptions struct {
	// DryRun computes the report without writing.
	DryRun bool
	// EmitOutbox writes outbox entries for the imported records, so that a publisher rebuilds the Redis keys from
	// PostgreSQL (restoring a lost Redis). Default: false, Redis already has the data.
	EmitOutbox bool
	// Target is the current PostgreSQL state; nil (dry-run without a database) treats the target as empty.
	Target *Data
}

type syncer struct {
	ctx    context.Context
	dry    bool
	report *Report
}

func (s *syncer) fam(f Family) *FamilyReport { return s.report.Families[f] }

func (s *syncer) fail(f Family, key string, err error) {
	s.fam(f).Errors++
	s.report.Errors = append(s.report.Errors, fmt.Sprintf("%s %q: %v", f, key, err))
}

// apply compares canonical values and writes the differences with write.
func (s *syncer) apply(f Family, src, dst map[string]string, write func(key string) error) {
	fr := s.fam(f)
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cur, ok := dst[k]
		if ok && cur == src[k] {
			fr.Unchanged++
			continue
		}
		if !s.dry {
			if err := write(k); err != nil {
				s.fail(f, k, err)
				continue
			}
		}
		if ok {
			fr.Updated++
		} else {
			fr.Created++
		}
	}
	for k := range dst {
		if _, ok := src[k]; !ok {
			fr.ExtraInTarget++
		}
	}
}

func index[T any](vs []T, key func(T) string) map[string]T {
	m := make(map[string]T, len(vs))
	for _, v := range vs {
		m[key(v)] = v
	}
	return m
}

// Import copies src to the store. It is idempotent: records that are already equal are not touched (no audit rows, no
// outbox entries), records that differ are overwritten with the Redis value, records only in PostgreSQL are kept.
// Imports do not write outbox entries: Redis already has the data. Write errors are collected in the report.
func Import(ctx context.Context, store controlplane.Store, src *Data, opt ImportOptions) *Report {
	rep := &Report{StartedAt: time.Now().UTC(), DryRun: opt.DryRun, Families: map[Family]*FamilyReport{}, Warnings: append([]string(nil), src.Warnings...)}
	for _, f := range Families {
		rep.Families[f] = &FamilyReport{}
	}
	dstData := opt.Target
	if dstData == nil {
		dstData = &Data{}
	}
	srcSnap, dstSnap := src.Canonical(), dstData.Canonical()
	for f, sum := range srcSnap.Summarize() {
		rep.Families[f].Source = sum
	}
	for f, sum := range dstSnap.Summarize() {
		rep.Families[f].Target = sum
	}
	ctx = controlplane.WithActor(ctx, controlplane.Actor{Name: Actor})
	if !opt.EmitOutbox {
		ctx = controlplane.WithSkipOutbox(ctx)
	}
	var r controlplane.Repos // zero in a dry run without a database: no write closure runs
	if store != nil {
		r = store.Repos()
	}
	s := &syncer{ctx: ctx, dry: opt.DryRun, report: rep}

	byName := index(src.Storages, func(v controlplane.StorageRecord) string { return v.Name })
	s.apply(FamStorages, srcSnap[FamStorages], dstSnap[FamStorages], func(k string) error { _, err := r.Storages.Put(ctx, byName[k]); return err })
	ids := index(src.S3Identities, func(v controlplane.S3Identity) string { return v.Name })
	s.apply(FamS3Identities, srcSnap[FamS3Identities], dstSnap[FamS3Identities], func(k string) error { _, err := r.S3Identities.Put(ctx, ids[k]); return err })
	if src.DynamicSalt != "" {
		s.apply(FamDynamicSalt, srcSnap[FamDynamicSalt], dstSnap[FamDynamicSalt], func(string) error {
			stored, err := r.DynamicCredentials.SetSaltIfAbsent(ctx, src.DynamicSalt)
			if err == nil && !stored {
				err = fmt.Errorf("a different salt is already stored in PostgreSQL; refusing to replace it (sealed values would become unreadable)")
			}
			return err
		})
	}
	creds := index(src.DynamicCreds, func(v controlplane.DynamicCredential) string { return v.RedisField() })
	s.apply(FamDynamicCreds, srcSnap[FamDynamicCreds], dstSnap[FamDynamicCreds], func(k string) error { return r.DynamicCredentials.Set(ctx, creds[k]) })
	pa := index(src.ProxyAccounts, func(v controlplane.ProxyAccountRecord) string { return v.AccessKeyID })
	s.apply(FamProxyAccounts, srcSnap[FamProxyAccounts], dstSnap[FamProxyAccounts], func(k string) error { _, err := r.ProxyAccounts.Put(ctx, pa[k]); return err })
	ap := index(src.AccessPolicies, func(v controlplane.AccessPolicy) string { return v.User })
	s.apply(FamAccessPolicies, srcSnap[FamAccessPolicies], dstSnap[FamAccessPolicies], func(k string) error { _, err := r.AccessPolicies.Put(ctx, ap[k]); return err })
	cu := index(src.ConsoleUsers, func(v controlplane.ConsoleUser) string { return v.Username })
	s.apply(FamConsoleUsers, srcSnap[FamConsoleUsers], dstSnap[FamConsoleUsers], func(k string) error { _, err := r.ConsoleUsers.Put(ctx, cu[k]); return err })
	s.apply(FamConsoleLogins, srcSnap[FamConsoleLogins], dstSnap[FamConsoleLogins], func(k string) error { return r.ConsoleUsers.TouchLogin(ctx, k, src.ConsoleLogins[k]) })

	routes := map[string]controlplane.RouteEntry{}
	for _, e := range src.Routing.Routes {
		routes[scopeKey(e.User, e.Bucket)] = e
	}
	s.apply(FamRoutes, srcSnap[FamRoutes], dstSnap[FamRoutes], func(k string) error {
		e := routes[k]
		return r.Routing.Set(ctx, e.User, e.Bucket, e.Storage)
	})
	blocks := map[string]controlplane.RouteEntry{}
	for _, e := range src.Routing.Blocks {
		blocks[scopeKey(e.User, e.Bucket)] = e
	}
	s.apply(FamRouteBlocks, srcSnap[FamRouteBlocks], dstSnap[FamRouteBlocks], func(k string) error {
		e := blocks[k]
		return r.Routing.Block(ctx, e.User, e.Bucket)
	})

	// statuses first (archived ones have no policy), then the policies (Add rewrites the same status: no-op)
	s.apply(FamReplStatus, srcSnap[FamReplStatus], dstSnap[FamReplStatus], func(k string) error {
		id, _ := controlplane.ParseReplicationID(k)
		return r.Replications.PutStatus(ctx, id, src.ReplStatuses[id])
	})
	pols := map[string]controlplane.ReplicationID{}
	for _, id := range src.ReplPolicies {
		pols[tkey(id)] = id
	}
	s.apply(FamReplPolicies, srcSnap[FamReplPolicies], dstSnap[FamReplPolicies], func(k string) error {
		id := pols[k]
		st, ok := src.ReplStatuses[id]
		if !ok {
			st = controlplane.ReplicationStatus{CreatedAt: time.Now().UTC()}
			s.report.Warnings = append(s.report.Warnings, fmt.Sprintf("replication %s has a policy but no status hash; created a status dated now", id.String()))
		}
		return r.Replications.Add(ctx, id, st)
	})
	gens := map[string]controlplane.ReplicationID{}
	for id := range src.ReplGenerations {
		gens[tkey(id)] = id
	}
	s.apply(FamReplGenerations, srcSnap[FamReplGenerations], dstSnap[FamReplGenerations], func(k string) error {
		id := gens[k]
		return r.Replications.SetGeneration(ctx, id, src.ReplGenerations[gens[k]])
	})

	fp := index(src.FailoverPolicies, func(v controlplane.FailoverPolicy) string { return v.Source })
	s.apply(FamFailoverPolicies, srcSnap[FamFailoverPolicies], dstSnap[FamFailoverPolicies], func(k string) error { _, err := r.FailoverPolicies.Put(ctx, fp[k]); return err })
	fe := index(src.FailoverEvents, func(v controlplane.FailoverEvent) string { return v.Key() })
	s.apply(FamFailoverEvents, srcSnap[FamFailoverEvents], dstSnap[FamFailoverEvents], func(k string) error { _, err := r.FailoverEvents.Put(ctx, fe[k]); return err })
	pg := index(src.ProtectionGroups, func(v controlplane.RawDoc) string { return v.ID })
	s.apply(FamProtectionGroups, srcSnap[FamProtectionGroups], dstSnap[FamProtectionGroups], func(k string) error { _, err := r.ProtectionGroups.Put(ctx, pg[k]); return err })
	tests := index(src.StorageTests, func(v controlplane.RawDoc) string { return v.ID })
	s.apply(FamStorageTests, srcSnap[FamStorageTests], dstSnap[FamStorageTests], func(k string) error { _, err := r.StorageTests.Put(ctx, tests[k]); return err })

	if !opt.DryRun {
		after, err := ReadPG(ctx, r)
		if err != nil {
			rep.Errors = append(rep.Errors, "re-reading PostgreSQL: "+err.Error())
		} else {
			afterSnap := after.Canonical()
			for f, sum := range afterSnap.Summarize() {
				sum := sum
				rep.Families[f].After = &sum
			}
			rep.Mismatches = mismatchesAgainstSource(srcSnap, afterSnap)
		}
	}
	rep.OK = len(rep.Errors) == 0 && len(rep.Mismatches) == 0
	return rep
}

// mismatchesAgainstSource ignores records that exist only in PostgreSQL (kept by design).
func mismatchesAgainstSource(src, dst Snapshot) []Diff {
	var res []Diff
	for _, d := range Compare(src, dst) {
		if d.Kind != "missing_in_source" {
			res = append(res, d)
		}
	}
	return res
}

// Verify compares Redis and PostgreSQL and fails on any difference, including records only in PostgreSQL.
func Verify(source, target *Data) *Report {
	rep := &Report{StartedAt: time.Now().UTC(), Families: map[Family]*FamilyReport{}, Warnings: append([]string(nil), source.Warnings...)}
	ss, ts := source.Canonical(), target.Canonical()
	srcSum, dstSum := ss.Summarize(), ts.Summarize()
	for _, f := range Families {
		rep.Families[f] = &FamilyReport{Source: srcSum[f], Target: dstSum[f]}
	}
	rep.Mismatches = Compare(ss, ts)
	for _, d := range rep.Mismatches {
		if d.Kind == "missing_in_source" {
			rep.Families[d.Family].ExtraInTarget++
		}
	}
	for _, f := range Families {
		fr := rep.Families[f]
		fr.Unchanged = fr.Source.Count
	}
	for _, d := range rep.Mismatches {
		if d.Kind != "missing_in_source" {
			rep.Families[d.Family].Unchanged--
		}
	}
	rep.OK = len(rep.Mismatches) == 0
	return rep
}

// JSON renders the report.
func (r *Report) JSON() []byte {
	b, _ := json.MarshalIndent(r, "", "  ")
	return b
}

// Text renders the report for a terminal.
func (r *Report) Text() string {
	out := fmt.Sprintf("started %s dry-run=%v\n", r.StartedAt.Format(time.RFC3339), r.DryRun)
	out += fmt.Sprintf("%-26s %7s %7s %7s %7s %7s  %s\n", "family", "redis", "pg", "created", "updated", "extra", "checksum (redis / pg after)")
	for _, f := range Families {
		fr := r.Families[f]
		if fr == nil {
			continue
		}
		pgSum := fr.Target
		if fr.After != nil {
			pgSum = *fr.After
		}
		match := "OK"
		if fr.Source.SHA != pgSum.SHA {
			match = "DIFF"
		}
		out += fmt.Sprintf("%-26s %7d %7d %7d %7d %7d  %s %.12s / %.12s\n", f, fr.Source.Count, pgSum.Count, fr.Created, fr.Updated, fr.ExtraInTarget, match, fr.Source.SHA, pgSum.SHA)
	}
	for _, w := range r.Warnings {
		out += "warning: " + w + "\n"
	}
	for _, e := range r.Errors {
		out += "error: " + e + "\n"
	}
	for _, m := range r.Mismatches {
		out += fmt.Sprintf("mismatch: %s %q %s\n", m.Family, m.Key, m.Kind)
	}
	if r.OK {
		out += "result: OK\n"
	} else {
		out += "result: FAILED\n"
	}
	return out
}
