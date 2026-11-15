/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { nextTick, ref } from 'vue';
import { useAccessPolicyForm } from '../useAccessPolicyForm';
import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import { BucketDiscoveryHelper } from '@/utils/helpers/BucketDiscoveryHelper';
import type { TuttiProxyAccount } from '@/utils/types/tutti';

vi.mock('@/utils/helpers/BucketDiscoveryHelper', () => ({
  BucketDiscoveryHelper: {
    discoverBuckets: vi.fn(),
    discoverStorageBuckets: vi.fn(),
  },
}));

function setup(account: TuttiProxyAccount) {
  const close = vi.fn();
  const show = ref(false);
  const policy = useAccessPolicyForm(show, ref(account), close);

  show.value = true;

  return { policy, close };
}

beforeEach(() => {
  setActivePinia(createPinia());

  const protection = useTuttiProtectionStore();

  protection.isLoaded = true;
  protection.groups = [
    { id: 'g1', buckets: ['a', 'b'] },
    { id: 'all-main', primary: 'main', members: ['main'] },
  ] as never;
  vi.mocked(BucketDiscoveryHelper.discoverBuckets).mockResolvedValue(['a']);
  vi.mocked(BucketDiscoveryHelper.discoverStorageBuckets).mockResolvedValue([
    { bucket: 'x', storage: 'main' },
  ]);
});

describe('useAccessPolicyForm', () => {
  it('opens in "all buckets" for a user without policy', async () => {
    const { policy } = setup({ user: 'alice' });

    await flushPromises();
    expect(policy.grantMode.value).toBe('all');
    expect(policy.clusterBuckets.value).toEqual(['a']);
  });

  it('restores group mode and splits extra patterns from group buckets', async () => {
    const { policy } = setup({
      user: 'alice',
      hasPolicy: true,
      policy: { user: 'alice', permission: 'read-only', buckets: ['a', 'x-*'] },
    } as never);

    await flushPromises();
    expect(policy.grantMode.value).toBe('groups');
    expect(policy.selectedGroupIds.value).toEqual(['g1']);
    expect(policy.selectedGroupBuckets.value).toEqual(['a']);
    expect(policy.extraBuckets.value).toEqual(['x-*']);
  });

  it('a storage-level group with nothing ticked saves every bucket', async () => {
    const { policy } = setup({ user: 'alice' });

    await flushPromises();
    policy.setGrantMode('groups');
    await policy.setGroups(['all-main']);
    policy.setAllGroupBuckets(false);
    expect(policy.scopeGroups.value).toHaveLength(1);
    expect(policy.form.value.buckets).toEqual(['*']);
  });

  it('rejects an invalid pending pattern and saves a valid one', async () => {
    const { policy, close } = setup({ user: 'alice' });
    const store = useTuttiAccessPolicyStore();

    store.savePolicy = vi.fn().mockResolvedValue(true);
    await flushPromises();
    policy.setGrantMode('custom');
    policy.customInput.value = 'has space';
    await nextTick();
    expect(policy.patternError.value).toBe('PATTERN_WHITESPACE');
    await policy.submit();
    expect(store.savePolicy).not.toHaveBeenCalled();

    policy.customInput.value = 'logs-*';
    await policy.submit();
    expect(store.savePolicy).toHaveBeenCalledWith(
      expect.objectContaining({ buckets: ['logs-*'] }),
    );
    expect(close).toHaveBeenCalled();
  });
});
