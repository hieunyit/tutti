/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useTuttiProxyFleetConfigStore } from '../tuttiProxyFleetConfigStore';
import { TuttiService } from '@/services/TuttiService';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getProxyFleetConfig: vi.fn(),
    setProxyFleetConfig: vi.fn(),
  },
}));

const loaded = {
  version: 4,
  defaults: { writerLease: '30s' },
  groups: { edge: { health: { failureThreshold: 3 } } },
};
const newer = { ...loaded, version: 6, defaults: { writerLease: '45s' } };
const conflict = { response: { data: { code: 9 } } };

async function conflicted() {
  vi.mocked(TuttiService.getProxyFleetConfig).mockResolvedValue(loaded);
  vi.mocked(TuttiService.setProxyFleetConfig).mockRejectedValue(conflict);
  const store = useTuttiProxyFleetConfigStore();

  await store.load();
  store.draft.defaults.writerLease = '60s';
  await store.save();

  return store;
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.mocked(TuttiService.getProxyFleetConfig).mockReset();
  vi.mocked(TuttiService.setProxyFleetConfig).mockReset();
});

describe('fleet config save conflict', () => {
  it('keeps the draft and the loaded version and sets the conflict state', async () => {
    const store = await conflicted();

    expect(store.conflict).toBe(true);
    expect(store.saveError).toBe(false);
    expect(store.saved).toBe(false);
    expect(store.saving).toBe(false);
    expect(store.draft.defaults.writerLease).toBe('60s');
    expect(store.version).toBe(4);
    expect(store.config?.defaults?.writerLease).toBe('30s');
  });

  it('a plain reload keeps the unsaved edits and does not refetch', async () => {
    const store = await conflicted();

    vi.mocked(TuttiService.getProxyFleetConfig).mockClear();
    await store.load();

    expect(TuttiService.getProxyFleetConfig).not.toHaveBeenCalled();
    expect(store.draft.defaults.writerLease).toBe('60s');
    expect(store.dirty).toBe(true);
  });

  it('only an explicit forced reload drops the edits, adopts the new version and clears the conflict', async () => {
    const store = await conflicted();

    vi.mocked(TuttiService.getProxyFleetConfig).mockResolvedValue(newer);
    await store.load(true);

    expect(store.conflict).toBe(false);
    expect(store.version).toBe(6);
    expect(store.draft.defaults.writerLease).toBe('45s');
    expect(store.dirty).toBe(false);
  });

  it('a failed forced reload keeps the edits and flags a load error', async () => {
    const store = await conflicted();

    vi.mocked(TuttiService.getProxyFleetConfig).mockRejectedValue(
      new Error('down'),
    );
    await store.load(true);

    expect(store.loadError).toBe(true);
    expect(store.draft.defaults.writerLease).toBe('60s');
  });

  it('reset discards the edits and the conflict without refetching', async () => {
    const store = await conflicted();

    store.reset();

    expect(store.conflict).toBe(false);
    expect(store.draft.defaults.writerLease).toBe('30s');
    expect(store.version).toBe(4);
  });

  it('a non-conflict failure sets saveError instead of conflict and keeps the draft', async () => {
    vi.mocked(TuttiService.getProxyFleetConfig).mockResolvedValue(loaded);
    vi.mocked(TuttiService.setProxyFleetConfig).mockRejectedValue({
      response: { data: { code: 13 } },
    });
    const store = useTuttiProxyFleetConfigStore();

    await store.load();
    store.draft.defaults.writerLease = '60s';
    await store.save();

    expect(store.saveError).toBe(true);
    expect(store.conflict).toBe(false);
    expect(store.draft.defaults.writerLease).toBe('60s');
  });

  it('saving again after a conflict still sends the stale version (the server decides)', async () => {
    const store = await conflicted();

    await store.save();

    expect(TuttiService.setProxyFleetConfig).toHaveBeenCalledTimes(2);
    expect(
      vi.mocked(TuttiService.setProxyFleetConfig).mock.calls[1]![0]
        .expectedVersion,
    ).toBe(4);
  });
});
