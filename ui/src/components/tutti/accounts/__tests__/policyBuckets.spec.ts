/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { describe, expect, it } from 'vitest';
import {
  availableGroupBuckets,
  bucketsFromGroups,
  grantStateFor,
  matchingGroups,
  nextGroupBuckets,
  storageScopeGroups,
  storagesToQuery,
} from '../policyBuckets';
import type { ProtectionGroup } from '@/utils/types/protection';

const bucketGroup = { id: 'g1', buckets: ['a', 'b'] } as ProtectionGroup;
const storageGroup = {
  id: 'g2',
  primary: 'main',
  members: ['main', 'replica'],
} as ProtectionGroup;
const groups = [bucketGroup, storageGroup];

describe('policyBuckets', () => {
  it('finds storage-scope groups and the storages to query', () => {
    const scope = storageScopeGroups(groups, ['g1', 'g2']);

    expect(scope.map((g) => g.id)).toEqual(['g2']);
    expect(storagesToQuery(scope)).toEqual(['main', 'replica']);
  });

  it('lists group buckets first, then discovered ones, without duplicates', () => {
    const items = availableGroupBuckets(
      groups,
      ['g1', 'g2'],
      [
        { bucket: 'a', storage: 'main' },
        { bucket: 'c', storage: 'main' },
      ],
    );

    expect(items).toEqual([
      { bucket: 'a', groupId: 'g1' },
      { bucket: 'b', groupId: 'g1' },
      { bucket: 'c', groupId: '', storage: 'main' },
    ]);
  });

  it('a storage-scope group with nothing ticked means every bucket', () => {
    expect(bucketsFromGroups([storageGroup], [], [])).toEqual(['*']);
    expect(bucketsFromGroups([storageGroup], ['c'], ['x-*'])).toEqual([
      'c',
      'x-*',
    ]);
    expect(bucketsFromGroups([], [], [])).toEqual([]);
  });

  it('keeps ticked buckets that still exist and ticks new ones', () => {
    const next = nextGroupBuckets(
      groups,
      ['g1'],
      ['g1', 'g2'],
      ['a', 'gone'],
      [{ bucket: 'c', storage: 'main' }],
    );

    expect(next).toEqual(['a', 'c']);
  });

  it('derives the grant mode of an existing policy', () => {
    expect(grantStateFor([], [], []).mode).toBe('all');
    expect(grantStateFor(['*'], [], []).mode).toBe('all');
    expect(grantStateFor(['logs-*'], [], []).mode).toBe('custom');

    const matching = matchingGroups(['a', 'extra-*'], groups);
    const state = grantStateFor(['a', 'extra-*'], matching, []);

    expect(state).toEqual({
      mode: 'groups',
      groupIds: ['g1'],
      groupBuckets: ['a'],
      extraBuckets: ['extra-*'],
    });
  });
});
