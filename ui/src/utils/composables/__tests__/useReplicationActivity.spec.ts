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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, ref, type Ref } from 'vue';
import { mount } from '@vue/test-utils';
import { useReplicationActivity } from '../useReplicationActivity';
import type {
  ReplicationActivityEvent,
  ReplicationActivityRequest,
  ReplicationActivityResponse,
} from '@/utils/types/replicationActivity';
import type { TuttiReplicationId } from '@/utils/types/tuttiCore';

vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: { getReplicationActivity: vi.fn() },
}));

const id: TuttiReplicationId = { fromStorage: 'a', toStorage: 'b' };

function ev(cursor: string): ReplicationActivityEvent {
  return {
    cursor,
    time: '2026-11-15T10:00:01Z',
    level: 'info',
    type: 'done',
    key: `k${cursor}`,
  };
}

function setup(
  fetcher: (
    p: ReplicationActivityRequest,
  ) => Promise<ReplicationActivityResponse>,
  maxEvents = 5,
) {
  const idRef: Ref<TuttiReplicationId | null> = ref(id);
  let api!: ReturnType<typeof useReplicationActivity>;
  const wrapper = mount(
    defineComponent({
      setup() {
        api = useReplicationActivity(idRef, {
          intervalMs: 2000,
          maxEvents,
          fetcher,
        });

        return () => h('div');
      },
    }),
  );

  return { api, wrapper };
}

describe('useReplicationActivity', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('polls with the cursor and merges without duplicates in order', async () => {
    const calls: ReplicationActivityRequest[] = [];
    const responses: ReplicationActivityResponse[] = [
      { events: [ev('1-0'), ev('2-0')], nextCursor: '2-0', inFlight: [] },
      { events: [ev('2-0'), ev('3-0')], nextCursor: '3-0' },
      { events: [], nextCursor: '3-0' },
    ];
    const { api } = setup(async (p) => {
      calls.push(JSON.parse(JSON.stringify(p)));

      return responses[Math.min(calls.length - 1, responses.length - 1)]!;
    });

    await api.start();
    expect(api.events.value.map((e) => e.cursor)).toEqual(['1-0', '2-0']);
    expect(calls[0]!.afterCursor).toBeUndefined();

    await vi.advanceTimersByTimeAsync(2000);
    expect(api.events.value.map((e) => e.cursor)).toEqual([
      '1-0',
      '2-0',
      '3-0',
    ]);
    expect(calls[1]!.afterCursor).toBe('2-0');

    await vi.advanceTimersByTimeAsync(2000);
    expect(calls[2]!.afterCursor).toBe('3-0');
    expect(api.events.value).toHaveLength(3);
  });

  it('caps the lines in memory', async () => {
    const { api } = setup(async () => ({
      events: Array.from({ length: 8 }, (_, i) => ev(`${i + 1}-0`)),
      nextCursor: '8-0',
    }));

    await api.start();
    expect(api.events.value.map((e) => e.cursor)).toEqual([
      '4-0',
      '5-0',
      '6-0',
      '7-0',
      '8-0',
    ]);
  });

  it('reports errors, keeps the lines and recovers on retry', async () => {
    let fail = false;
    const { api } = setup(async () => {
      if (fail) throw new Error('network');

      return { events: [ev('1-0')], nextCursor: '1-0' };
    });

    await api.start();
    fail = true;
    await vi.advanceTimersByTimeAsync(2000);
    expect(api.error.value).toBe(true);
    expect(api.events.value).toHaveLength(1);

    fail = false;
    await api.retry();
    expect(api.error.value).toBe(false);
  });

  it('stops polling when live is switched off and on unmount', async () => {
    const fetcher = vi.fn().mockResolvedValue({ events: [] });
    const { api, wrapper } = setup(fetcher);

    await api.start();
    expect(fetcher).toHaveBeenCalledTimes(1);

    api.live.value = false;
    await vi.advanceTimersByTimeAsync(10_000);
    expect(fetcher).toHaveBeenCalledTimes(1);

    api.live.value = true;
    await vi.advanceTimersByTimeAsync(0);
    expect(fetcher).toHaveBeenCalledTimes(2);

    wrapper.unmount();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it('clear drops the lines but not the cursor', async () => {
    const calls: ReplicationActivityRequest[] = [];
    const { api } = setup(async (p) => {
      calls.push(p);

      return {
        events: calls.length === 1 ? [ev('1-0')] : [],
        nextCursor: '1-0',
      };
    });

    await api.start();
    api.clear();
    expect(api.events.value).toEqual([]);

    await vi.advanceTimersByTimeAsync(2000);
    expect(calls[1]!.afterCursor).toBe('1-0');
    expect(api.events.value).toEqual([]);
  });
});
