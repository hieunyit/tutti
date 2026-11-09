/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiFailoverStore } from '../tuttiFailoverStore';
import {
  HEALTH_REFRESH_INTERVAL_MS,
  useTuttiHealthStore,
} from '../tuttiHealthStore';
import { TuttiService } from '@/services/TuttiService';
import { ChorusService } from '@/services/ChorusService';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getProxies: vi.fn(),
    getFailoverPolicies: vi.fn(),
    getFailoverEvents: vi.fn(),
    setFailoverPolicy: vi.fn(),
    deleteFailoverPolicy: vi.fn(),
    deleteFailoverEvent: vi.fn(),
  },
}));
vi.mock('@/services/ChorusService', () => ({
  ChorusService: { getStorages: vi.fn() },
}));

type MockFn = ReturnType<typeof vi.fn>;

const svc = TuttiService as unknown as {
  [K in
    | 'getProxies'
    | 'getFailoverPolicies'
    | 'getFailoverEvents'
    | 'setFailoverPolicy'
    | 'deleteFailoverPolicy'
    | 'deleteFailoverEvent']: MockFn;
};
const chorus = ChorusService as unknown as { getStorages: MockFn };

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetAllMocks();
});

describe('tuttiHealthStore', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('loads proxies and treats missing list as empty', async () => {
    const store = useTuttiHealthStore();

    svc.getProxies.mockResolvedValue({});
    await store.loadProxies();
    expect(store.proxies).toEqual([]);
    expect(store.hasNoData).toBe(true);

    svc.getProxies.mockResolvedValue({ proxies: [{ appId: 'a' }] });
    await store.loadProxies();
    expect(store.hasNoData).toBe(false);
  });

  it('sets hasError on failure', async () => {
    const store = useTuttiHealthStore();

    svc.getProxies.mockRejectedValue(new Error('x'));
    await store.loadProxies();
    expect(store.hasError).toBe(true);
  });

  it('polls every 10 s and stops on reset', async () => {
    vi.useFakeTimers();
    svc.getProxies.mockResolvedValue({ proxies: [] });
    const store = useTuttiHealthStore();

    store.startPolling();
    expect(svc.getProxies).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(HEALTH_REFRESH_INTERVAL_MS);
    expect(svc.getProxies).toHaveBeenCalledTimes(2);
    store.$reset();
    await vi.advanceTimersByTimeAsync(HEALTH_REFRESH_INTERVAL_MS * 3);
    expect(svc.getProxies).toHaveBeenCalledTimes(2);
  });
});

describe('tuttiFailoverStore', () => {
  it('inits all data and counts proxies with failover disabled', async () => {
    svc.getFailoverPolicies.mockResolvedValue({
      policies: [{ source: 'a', targets: ['b'] }],
    });
    svc.getFailoverEvents.mockResolvedValue({});
    svc.getProxies.mockResolvedValue({
      proxies: [
        { appId: '1', settings: { failover: { enabled: true } } },
        { appId: '2', settings: { failover: {} } },
        { appId: '3' },
      ],
    });
    chorus.getStorages.mockResolvedValue({
      storages: [{ name: 'a' }, { name: 'b' }],
    });
    const store = useTuttiFailoverStore();

    await store.init();
    expect(store.policies).toHaveLength(1);
    expect(store.events).toEqual([]);
    expect(store.storageNames).toEqual(['a', 'b']);
    expect(store.proxiesWithFailoverDisabled).toBe(2);
    expect(store.hasError).toBe(false);
  });

  it('flags error when init fails', async () => {
    svc.getFailoverPolicies.mockRejectedValue(new Error('x'));
    svc.getFailoverEvents.mockResolvedValue({});
    svc.getProxies.mockResolvedValue({});
    chorus.getStorages.mockResolvedValue({});
    const store = useTuttiFailoverStore();

    await store.init();
    expect(store.hasError).toBe(true);
  });

  it('saves a policy and reloads the list', async () => {
    svc.setFailoverPolicy.mockResolvedValue(undefined);
    svc.getFailoverPolicies.mockResolvedValue({
      policies: [{ source: 'a', targets: ['b'] }],
    });
    const store = useTuttiFailoverStore();

    const ok = await store.savePolicy({
      source: 'a',
      enabled: true,
      targets: ['b'],
      after: '60',
      maxEventLag: null,
    });

    expect(ok).toBe(true);
    expect(svc.setFailoverPolicy).toHaveBeenCalledWith({
      source: 'a',
      enabled: true,
      targets: ['b'],
      after: '60s',
    });
    expect(store.policies).toHaveLength(1);
  });

  it('keeps unknown save errors generic', async () => {
    svc.setFailoverPolicy.mockRejectedValue(new Error('boom'));
    const store = useTuttiFailoverStore();

    const ok = await store.savePolicy({
      source: 'a',
      enabled: false,
      targets: [],
      after: '',
      maxEventLag: null,
    });

    expect(ok).toBe(false);
    expect(store.saveError).toBe('unknown');
  });

  it('deletes a policy and acknowledges an event', async () => {
    svc.deleteFailoverPolicy.mockResolvedValue(undefined);
    svc.getFailoverPolicies.mockResolvedValue({});
    svc.deleteFailoverEvent.mockResolvedValue(undefined);
    svc.getFailoverEvents.mockResolvedValue({});
    const store = useTuttiFailoverStore();

    expect(await store.deletePolicy('a')).toBe(true);
    expect(svc.deleteFailoverPolicy).toHaveBeenCalledWith('a');
    expect(
      await store.acknowledgeEvent({ user: 'u', from: 'a', to: 'b' }),
    ).toBe(true);
    expect(svc.deleteFailoverEvent).toHaveBeenCalledWith({
      user: 'u',
      bucket: '',
    });
  });

  it('returns false when delete fails', async () => {
    svc.deleteFailoverPolicy.mockRejectedValue(new Error('x'));
    const store = useTuttiFailoverStore();

    expect(await store.deletePolicy('a')).toBe(false);
  });
});
