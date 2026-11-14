<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->

<script setup lang="ts">
  /**
   * Live replication throughput: rate/buffer tiles, plain-language hints and a
   * per-worker table. Hidden when the backend has no worker-stats route.
   */
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import messages from './i18nThroughput';
  import { ErrorBlock, LoadingBlock, StatTile } from '@/components/kit';
  import { useTuttiWorkerStatsStore } from '@/stores/tuttiWorkerStatsStore';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import {
    bufferPercent,
    formatBytesPerSec,
    formatObjectsPerSec,
  } from '@/utils/helpers/ThroughputHelper';

  withDefaults(defineProps<{ showSettingsLink?: boolean }>(), {
    showSettingsLink: true,
  });

  const { t } = useI18n({ messages });
  const store = useTuttiWorkerStatsStore();
  const { isLoading, hasError, unavailable, totals, workers, hints } =
    storeToRefs(store);

  onBeforeMount(() => {
    void store.start();
  });
  onBeforeUnmount(() => {
    void store.stop();
  });

  const percent = computed(() =>
    totals.value ? bufferPercent(totals.value) : null,
  );
  const bufferValue = computed(() =>
    percent.value === null ? '–' : `${percent.value}%`,
  );
  const bufferHint = computed(() =>
    totals.value && percent.value !== null
      ? t('bufferHint', {
          used: TrafficStatsHelper.formatBytes(totals.value.bufferUsedBytes),
          total: TrafficStatsHelper.formatBytes(totals.value.bufferTotalBytes),
        })
      : t('bufferNone'),
  );
  const bufferTone = computed(() =>
    percent.value !== null && percent.value >= 90 ? 'warning' : 'neutral',
  );

  function workerBuffer(w: {
    bufferTotalBytes: number;
    bufferUsedBytes: number;
  }) {
    const p = bufferPercent(w);

    return p === null ? '–' : `${p}%`;
  }
</script>

<template>
  <section
    v-if="!unavailable"
    class="throughput"
    data-test="throughput-panel"
    :aria-label="t('title')"
  >
    <header class="throughput__head">
      <div>
        <h3 class="throughput__title">{{ t('title') }}</h3>
        <p class="throughput__desc">{{ t('description') }}</p>
      </div>
      <RouterLink
        v-if="showSettingsLink"
        class="throughput__link"
        to="/system-settings?tab=worker"
        >{{ t('settingsLink') }}</RouterLink
      >
    </header>

    <ErrorBlock
      v-if="hasError && !totals"
      :title="t('errorTitle')"
      :description="t('errorText')"
      @retry="store.retry()"
    />
    <LoadingBlock
      v-else-if="isLoading || !totals"
      :rows="3"
    />
    <template v-else>
      <div
        class="throughput__tiles"
        data-test="throughput-tiles"
      >
        <StatTile
          :label="t('objects1m')"
          :value="formatObjectsPerSec(totals.objectsPerSec1m)"
        />
        <StatTile
          :label="t('objects5m')"
          :value="formatObjectsPerSec(totals.objectsPerSec5m)"
        />
        <StatTile
          :label="t('bytes1m')"
          :value="formatBytesPerSec(totals.bytesPerSec1m)"
        />
        <StatTile
          :label="t('bytes5m')"
          :value="formatBytesPerSec(totals.bytesPerSec5m)"
        />
        <StatTile
          :label="t('inFlight')"
          :value="`${totals.inFlight} / ${totals.concurrency}`"
          :hint="t('inFlightHint')"
        />
        <StatTile
          :label="t('buffer')"
          :value="bufferValue"
          :hint="bufferHint"
          :tone="bufferTone"
        />
        <StatTile
          :label="t('waiters')"
          :value="totals.bufferWaiters"
          :hint="t('waitersHint')"
          :tone="totals.bufferWaiters > 0 ? 'warning' : 'neutral'"
        />
        <StatTile
          :label="t('failed')"
          :value="totals.failedTotal"
          :tone="totals.failedTotal > 0 ? 'warning' : 'neutral'"
        />
      </div>

      <div
        class="throughput__diagnostics"
        data-test="throughput-diagnostics"
        role="status"
      >
        <h4 class="throughput__subtitle">{{ t('diagnosticsTitle') }}</h4>
        <ul
          v-if="hints.length"
          class="throughput__hints"
        >
          <li
            v-for="hint in hints"
            :key="hint"
            :data-hint="hint"
          >
            {{ t(`hint_${hint}`) }}
          </li>
        </ul>
        <p
          v-else
          class="throughput__desc"
        >
          {{ t('diagnosticsOk') }}
        </p>
      </div>

      <h4 class="throughput__subtitle">{{ t('workersTitle') }}</h4>
      <p
        v-if="!workers.length"
        class="throughput__desc"
      >
        {{ t('noWorkers') }}
      </p>
      <div
        v-else
        class="throughput__table-wrap"
      >
        <table
          class="throughput__table"
          data-test="throughput-workers"
        >
          <thead>
            <tr>
              <th scope="col">{{ t('colWorker') }}</th>
              <th scope="col">{{ t('colInFlight') }}</th>
              <th scope="col">{{ t('colObjects') }}</th>
              <th scope="col">{{ t('colBytes') }}</th>
              <th scope="col">{{ t('colBuffer') }}</th>
              <th scope="col">{{ t('colFailed') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="w in workers"
              :key="w.workerId"
            >
              <th scope="row">
                <span class="tt-mono">{{ w.workerId }}</span>
              </th>
              <td>{{ w.inFlight }} / {{ w.concurrency }}</td>
              <td>{{ formatObjectsPerSec(w.objectsPerSec1m) }}</td>
              <td>{{ formatBytesPerSec(w.bytesPerSec1m) }}</td>
              <td>{{ workerBuffer(w) }}</td>
              <td>{{ w.failedTotal }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </section>
</template>

<style lang="scss" scoped>
  .throughput {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__head {
      display: flex;
      flex-wrap: wrap;
      align-items: flex-start;
      justify-content: space-between;
      gap: var(--tt-space-3);
    }

    &__title {
      margin: 0;
      font-size: var(--tt-text-lg);
      font-weight: var(--tt-weight-semibold);
    }

    &__subtitle {
      margin: 0;
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
    }

    &__desc {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__link {
      font-size: var(--tt-text-sm);
    }

    &__tiles {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
      gap: var(--tt-space-3);
    }

    &__diagnostics {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
    }

    &__hints {
      margin: 0;
      padding-left: var(--tt-space-5);
      font-size: var(--tt-text-sm);
    }

    &__table-wrap {
      overflow-x: auto;
    }

    &__table {
      width: 100%;
      border-collapse: collapse;
      font-size: var(--tt-text-sm);

      th,
      td {
        padding: var(--tt-space-2) var(--tt-space-3);
        text-align: left;
        border-bottom: 1px solid var(--tt-border-subtle);
        font-weight: var(--tt-weight-regular);
      }

      thead th {
        color: var(--tt-text-3);
        font-weight: var(--tt-weight-medium);
      }
    }
  }
</style>
