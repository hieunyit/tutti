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

import { onBeforeUnmount, ref, shallowRef, watch, type Ref } from 'vue';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { usePolling } from '@/utils/composables/usePolling';
import {
  ACTIVITY_MAX_EVENTS,
  ActivityLogHelper,
} from '@/utils/helpers/ActivityLogHelper';
import type {
  ReplicationActivityEvent,
  ReplicationActivityRequest,
  ReplicationActivityResponse,
  ReplicationInFlight,
} from '@/utils/types/replicationActivity';
import type { TuttiReplicationId } from '@/utils/types/tuttiCore';

export const ACTIVITY_POLL_MS = 2000;
const PAGE_LIMIT = 500;

export interface ReplicationActivityOptions {
  intervalMs?: number;
  maxEvents?: number;
  /** replaceable in tests */
  fetcher?: (
    payload: ReplicationActivityRequest,
  ) => Promise<ReplicationActivityResponse>;
}

/**
 * Live activity of one replication: polls with the server cursor (only new
 * events are requested), merges them without duplicates in cursor order and
 * keeps at most `maxEvents` lines. Polling pauses while the tab is hidden
 * (usePolling), can be switched off with `live`, and stops on unmount.
 */
export function useReplicationActivity(
  id: Ref<TuttiReplicationId | null>,
  options: ReplicationActivityOptions = {},
) {
  const {
    intervalMs = ACTIVITY_POLL_MS,
    maxEvents = ACTIVITY_MAX_EVENTS,
    fetcher = (payload) => TuttiCoreService.getReplicationActivity(payload),
  } = options;
  const polling = usePolling();

  const events = shallowRef<ReplicationActivityEvent[]>([]);
  const inFlight = shallowRef<ReplicationInFlight[]>([]);
  const serverTime = ref<string | null>(null);
  const loading = ref(false);
  const error = ref(false);
  const live = ref(true);
  let cursor = '';
  let generation = 0;

  async function tick() {
    const current = id.value;
    const gen = generation;

    if (!current) return;

    try {
      const res = await fetcher({
        id: current,
        afterCursor: cursor || undefined,
        limit: PAGE_LIMIT,
      });

      // the replication changed or the log was reset while the request ran
      if (gen !== generation) return;

      events.value = ActivityLogHelper.mergeEvents(
        events.value,
        res.events ?? [],
        maxEvents,
      );
      inFlight.value = res.inFlight ?? [];
      serverTime.value = res.serverTime ?? null;

      if (res.nextCursor) cursor = res.nextCursor;

      error.value = false;
    } catch {
      if (gen === generation) error.value = true;
    }
  }

  async function start() {
    if (!id.value) return;

    loading.value = events.value.length === 0 && !error.value;
    await polling.start(tick, intervalMs);
    loading.value = false;
  }

  async function stop() {
    await polling.stop();
  }

  /** drop the loaded lines (the cursor stays, they do not come back) */
  function clear() {
    events.value = [];
  }

  /** forget everything and read the newest events again */
  async function reset() {
    generation++;
    cursor = '';
    events.value = [];
    inFlight.value = [];
    serverTime.value = null;
    error.value = false;

    if (live.value) await start();
  }

  async function retry() {
    error.value = false;
    await polling.stop();
    await start();
  }

  watch(live, (on) => {
    if (on) void start();
    else void stop();
  });
  watch(id, (next, prev) => {
    if (JSON.stringify(next) !== JSON.stringify(prev)) void reset();
  });
  onBeforeUnmount(() => {
    generation++;
    void polling.stop();
  });

  return {
    events,
    inFlight,
    serverTime,
    loading,
    error,
    live,
    start,
    stop,
    clear,
    reset,
    retry,
  };
}
