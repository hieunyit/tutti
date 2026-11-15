/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
// the helpers and the i18n module import each other; start from the i18n side
import '@/i18n';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiRoutingPoliciesStore } from '../tuttiRoutingPoliciesStore';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { RoutingPolicyStatusFilter } from '@/utils/types/tuttiCore';

vi.mock('@/i18n/useStoreI18n', () => ({
  useStoreI18n: () => ({ t: (key: string) => key }),
}));
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));
vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: {
    getRoutingPolicies: vi.fn(),
    blockRoutingPolicy: vi.fn(),
    unblockRoutingPolicy: vi.fn(),
    deleteRoutingPolicy: vi.fn(),
    updateRoutingPolicy: vi.fn(),
  },
}));

const core = vi.mocked(TuttiCoreService);

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  core.getRoutingPolicies.mockResolvedValue({
    main: 'main',
    userRoutings: [{ user: 'alice', toStorage: 'main', isBlocked: false }],
    bucketRoutings: [
      { user: 'bob', bucket: 'photos', toStorage: 'backup', isBlocked: true },
    ],
  });
  core.blockRoutingPolicy.mockResolvedValue(undefined as never);
  core.unblockRoutingPolicy.mockResolvedValue(undefined as never);
  core.deleteRoutingPolicy.mockResolvedValue(undefined as never);
});

afterEach(async () => {
  await useTuttiRoutingPoliciesStore().$reset();
});

async function loaded() {
  const store = useTuttiRoutingPoliciesStore();

  await store.initRoutingPoliciesPage();

  return store;
}

describe('tuttiRoutingPoliciesStore', () => {
  it('lists user rules as all-buckets rules next to bucket rules', async () => {
    const store = await loaded();

    expect(
      store.computedRoutingPolicies.map((rule) => rule.bucket).sort(),
    ).toEqual(['*', 'photos']);
    expect(store.hasNoData).toBe(false);
  });

  it('filters by user, status and storage together', async () => {
    const store = await loaded();

    store.filterStatus = RoutingPolicyStatusFilter.BLOCKED;
    expect(store.computedRoutingPolicies.map((rule) => rule.user)).toEqual([
      'bob',
    ]);

    store.filterUsers = ['alice'];
    expect(store.computedRoutingPolicies).toEqual([]);
    expect(store.isFiltered).toBe(true);

    store.clearFilters();
    expect(store.computedRoutingPolicies).toHaveLength(2);
  });

  it('addresses an all-buckets rule with a null bucket when blocking', async () => {
    const store = await loaded();
    const alice = store.computedRoutingPolicies.find(
      (rule) => rule.user === 'alice',
    )!;

    const result = await store.setRoutingPoliciesBlock([alice], true);

    expect(core.blockRoutingPolicy).toHaveBeenCalledWith({
      user: 'alice',
      bucket: null,
    });
    expect(result.errorList).toEqual([]);
    expect(result.successList.map((rule) => rule.user)).toEqual(['alice']);
  });

  it('collects failures per rule and keeps the rest', async () => {
    const store = await loaded();

    core.deleteRoutingPolicy.mockImplementation(async ({ user }) => {
      if (user === 'bob') throw new Error('still blocked');
    });
    const result = await store.deleteRoutingPolicies([
      ...store.computedRoutingPolicies,
    ]);

    expect(result.successList.map((rule) => rule.user)).toEqual(['alice']);
    expect(result.errorList.map(([rule]) => rule.user)).toEqual(['bob']);
  });
});
