/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

/**
 * Pure bucket-rule logic of the access policy drawer (no Vue, no I/O), so the
 * storage-level replication scope rules stay unit-testable.
 */
import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
import type { StorageBucketEntry } from '@/utils/helpers/BucketDiscoveryHelper';
import type { ProtectionGroup } from '@/utils/types/protection';

export type GrantMode = 'all' | 'groups' | 'custom';

export interface GroupBucketItem {
  bucket: string;
  /** protection group id, or empty when the bucket comes from a storage */
  groupId: string;
  /** storage name when the bucket was discovered on a storage */
  storage?: string;
}

export interface GrantState {
  mode: GrantMode;
  groupIds: string[];
  groupBuckets: string[];
  extraBuckets: string[];
}

export function storageScopeGroups(
  groups: ProtectionGroup[],
  selectedIds: string[],
): ProtectionGroup[] {
  return groups.filter(
    (g) =>
      selectedIds.includes(g.id) &&
      ProtectionGroupHelper.scopeKind(g) === 'storage',
  );
}

/** Storages whose buckets must be discovered for the given storage-scope groups. */
export function storagesToQuery(groups: ProtectionGroup[]): string[] {
  const names = new Set<string>();

  for (const g of groups) {
    if (g.primary) names.add(g.primary);

    for (const m of g.members ?? []) names.add(m);
  }

  return [...names];
}

/** Buckets of the selected groups first, then buckets discovered on storages. */
export function availableGroupBuckets(
  groups: ProtectionGroup[],
  selectedIds: string[],
  discovered: StorageBucketEntry[],
): GroupBucketItem[] {
  const items: GroupBucketItem[] = [];
  const seen = new Set<string>();

  for (const g of groups.filter((candidate) =>
    selectedIds.includes(candidate.id),
  )) {
    for (const bucket of g.buckets ?? []) {
      if (!seen.has(bucket)) {
        seen.add(bucket);
        items.push({ bucket, groupId: g.id });
      }
    }
  }

  for (const entry of discovered) {
    if (!seen.has(entry.bucket)) {
      seen.add(entry.bucket);
      items.push({ bucket: entry.bucket, groupId: '', storage: entry.storage });
    }
  }

  return items;
}

/**
 * Patterns saved for the form. A storage-scope group with nothing ticked and
 * no extra patterns means "every bucket" (`*`).
 */
export function bucketsFromGroups(
  scopeGroups: ProtectionGroup[],
  groupBuckets: string[],
  extraBuckets: string[],
): string[] {
  if (
    scopeGroups.length > 0 &&
    groupBuckets.length === 0 &&
    extraBuckets.length === 0
  ) {
    return ['*'];
  }

  return [...groupBuckets, ...extraBuckets];
}

/** Buckets to tick after the group selection changed: kept ones plus new ones. */
export function nextGroupBuckets(
  groups: ProtectionGroup[],
  previousIds: string[],
  nextIds: string[],
  current: string[],
  discovered: StorageBucketEntry[],
): string[] {
  const available = new Set(
    availableGroupBuckets(groups, nextIds, discovered).map((b) => b.bucket),
  );
  const kept = current.filter((b) => available.has(b));
  const added: string[] = [];

  for (const id of nextIds.filter(
    (candidate) => !previousIds.includes(candidate),
  )) {
    for (const b of groups.find((g) => g.id === id)?.buckets ?? []) {
      if (!kept.includes(b) && !added.includes(b)) added.push(b);
    }
  }

  for (const entry of discovered) {
    if (!kept.includes(entry.bucket) && !added.includes(entry.bucket)) {
      added.push(entry.bucket);
    }
  }

  return [...kept, ...added];
}

/** Groups that protect at least one of the given patterns. */
export function matchingGroups(
  buckets: string[],
  groups: ProtectionGroup[],
): ProtectionGroup[] {
  return groups.filter((g) =>
    (g.buckets ?? []).some((b) => buckets.includes(b)),
  );
}

/** Which grant mode an existing list of patterns corresponds to. */
export function grantStateFor(
  buckets: string[],
  matching: ProtectionGroup[],
  discovered: StorageBucketEntry[],
): GrantState {
  if (buckets.length === 0 || buckets.includes('*')) {
    return { mode: 'all', groupIds: [], groupBuckets: [], extraBuckets: [] };
  }

  if (matching.length === 0) {
    return { mode: 'custom', groupIds: [], groupBuckets: [], extraBuckets: [] };
  }

  const known = new Set(matching.flatMap((g) => g.buckets ?? []));

  for (const entry of discovered) known.add(entry.bucket);

  return {
    mode: 'groups',
    groupIds: matching.map((g) => g.id),
    groupBuckets: buckets.filter((b) => known.has(b)),
    extraBuckets: buckets.filter((b) => !known.has(b)),
  };
}
