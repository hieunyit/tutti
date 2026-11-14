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

import { isAxiosError } from 'axios';
import { onBeforeUnmount, ref, shallowRef, watch, type Ref } from 'vue';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { usePolling } from '@/utils/composables/usePolling';
import type { AddId } from '@/utils/types/helper';
import {
  ReplicationType,
  type TuttiReplication,
  type TuttiReplicationId,
} from '@/utils/types/tuttiCore';

const POLL_MS = 5000;

/**
 * The replication shown on the detail page, refreshed every 5 s (paused while
 * the tab is hidden). `missing` is set when the server answers 404.
 */
export function useReplicationDetail(id: Ref<TuttiReplicationId | null>) {
  const polling = usePolling();
  const replication = shallowRef<AddId<TuttiReplication> | null>(null);
  const loading = ref(false);
  const error = ref(false);
  const missing = ref(false);
  let generation = 0;

  async function fetchOnce() {
    const current = id.value;
    const gen = generation;

    if (!current) return;

    try {
      const res = await TuttiCoreService.getReplication(current);

      if (gen !== generation) return;

      replication.value = {
        ...res,
        id: res.id ?? current,
        idStr: JSON.stringify(current),
        replicationType:
          current.fromBucket && current.toBucket
            ? ReplicationType.BUCKET
            : ReplicationType.USER,
      };
      error.value = false;
      missing.value = false;
    } catch (e) {
      if (gen !== generation) return;

      if (isAxiosError(e) && e.response?.status === 404) {
        missing.value = true;
        replication.value = null;
        await polling.stop();

        return;
      }

      error.value = true;
    }
  }

  async function load() {
    generation++;
    replication.value = null;
    missing.value = false;
    error.value = false;

    if (!id.value) return;

    loading.value = true;
    await polling.start(fetchOnce, POLL_MS);
    loading.value = false;
  }

  /** persists the name; rejects when the server refuses it */
  async function rename(name: string) {
    if (!id.value || !replication.value) return;

    await TuttiCoreService.renameReplication({ id: id.value, name });
    replication.value = { ...replication.value, name };
  }

  async function setPaused(paused: boolean) {
    if (!id.value) return;

    await (paused
      ? TuttiCoreService.pauseBucketReplication(id.value)
      : TuttiCoreService.resumeBucketReplication(id.value));
    await fetchOnce();
  }

  watch(
    id,
    (next, prev) => {
      if (JSON.stringify(next) !== JSON.stringify(prev)) void load();
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    generation++;
    void polling.stop();
  });

  return { replication, loading, error, missing, load, rename, setPaused };
}
