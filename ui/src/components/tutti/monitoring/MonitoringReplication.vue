/* * Copyright © 2026 hieunyit * * Licensed under the GNU Affero General Public
License, Version 3.0 (the "License"); * you may not use this file except in
compliance with the License. * You may obtain a copy of the License at * *
https://www.gnu.org/licenses/agpl-3.0.html * * Unless required by applicable law
or agreed to in writing, software * distributed under the License is distributed
on an "AS IS" BASIS, * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
express or implied. * See the License for the specific language governing
permissions and * limitations under the License. */

<script setup lang="ts">
  import { storeToRefs } from 'pinia';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import { useNumberFormat } from './useNumberFormat';
  import Callout from '@/components/ui/Callout.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nMonitoring });
  const fmt = useNumberFormat();
  const store = useTuttiMonitoringStore();
  const { replications, replicationsError, proxies } = storeToRefs(store);

  const num = (v: string | number | undefined) =>
    TrafficStatsHelper.toNumber(v);

  const sync = computed(() => {
    // depends on proxies: recomputed on every poll
    void proxies.value;

    return store.syncTotals();
  });
  const syncTotal = computed(
    () => sync.value.reached + sync.value.below + sync.value.skipped,
  );

  const laggingCount = computed(
    () => replications.value.filter((r) => store.isLagging(r)).length,
  );

  function label(replication: TuttiReplication): string {
    const { user, fromStorage, toStorage, fromBucket } = replication.id;

    return `${user}: ${fromStorage} → ${toStorage}${fromBucket ? ` (${fromBucket})` : ''}`;
  }

  function initText(r: TuttiReplication): string {
    if (r.isInitDone) return t('initDone');

    const listed = num(r.initObjListed);
    const done = num(r.initObjDone);

    return `${fmt(done)} / ${fmt(listed)}${
      listed > 0 ? ` (${Math.round((done / listed) * 100)} %)` : ''
    }`;
  }
</script>

<template>
  <div class="monitoring-replication">
    <SectionCard :title="t('syncTitle')">
      <p
        v-if="syncTotal === 0"
        class="monitoring-replication__empty"
      >
        {{ t('syncEmpty') }}
      </p>
      <template v-else>
        <dl class="monitoring-replication__sync">
          <div>
            <dt>{{ t('syncQuorumReached') }}</dt>
            <dd>{{ fmt(sync.reached) }}</dd>
          </div>
          <div>
            <dt>{{ t('syncBelowQuorum') }}</dt>
            <dd :class="{ 'monitoring-replication__bad': sync.below > 0 }">
              {{ fmt(sync.below) }}
            </dd>
          </div>
          <div>
            <dt>{{ t('syncSkipped') }}</dt>
            <dd>{{ fmt(sync.skipped) }}</dd>
          </div>
        </dl>
        <p class="monitoring-replication__hint">{{ t('syncHint') }}</p>
      </template>
    </SectionCard>

    <SectionCard
      :title="t('backlogTitle')"
      flush
    >
      <div
        v-if="replicationsError || laggingCount > 0"
        class="monitoring-replication__alerts"
      >
        <Callout
          v-if="replicationsError"
          tone="danger"
        >
          {{ t('backlogError') }}
        </Callout>
        <Callout
          v-if="laggingCount > 0"
          tone="warning"
          :title="t('lagWarning')"
        >
          {{ t('lagWarningText', { count: laggingCount }) }}
        </Callout>
      </div>

      <p
        v-if="replications.length === 0"
        class="monitoring-replication__empty monitoring-replication__empty--padded"
      >
        {{ t('backlogEmpty') }}
        <RouterLink :to="{ name: RouteName.TUTTI_REPLICATION }">
          {{ t('backlogLink') }}
        </RouterLink>
      </p>
      <table
        v-else
        class="tt-table"
      >
        <caption class="tt-sr-only">
          {{
            t('backlogTitle')
          }}
        </caption>
        <thead>
          <tr>
            <th scope="col">{{ t('columnReplication') }}</th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnEvents') }}
            </th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnEventsDone') }}
            </th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnLag') }}
            </th>
            <th scope="col">{{ t('columnInitSync') }}</th>
            <th scope="col">{{ t('columnState') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in replications"
            :key="r.idStr"
          >
            <td>{{ label(r) }}</td>
            <td class="tt-num">{{ fmt(num(r.events)) }}</td>
            <td class="tt-num">{{ fmt(num(r.eventsDone)) }}</td>
            <td class="tt-num">
              <StatusTag
                :variant="
                  num(r.eventLag) === 0
                    ? 'success'
                    : store.isLagging(r)
                      ? 'warning'
                      : 'neutral'
                "
                :label="fmt(num(r.eventLag))"
              />
            </td>
            <td>{{ initText(r) }}</td>
            <td>
              <StatusTag
                :variant="r.isPaused ? 'warning' : 'info'"
                :label="r.isPaused ? t('statePaused') : t('stateActive')"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </SectionCard>
  </div>
</template>

<style lang="scss" scoped>
  .monitoring-replication {
    display: grid;
    gap: 16px;

    &__sync {
      display: flex;
      flex-wrap: wrap;
      gap: 12px 40px;
      margin: 0;

      dt {
        font-size: 12px;
        color: var(--tt-text-3);
      }

      dd {
        margin: 2px 0 0;
        font-size: 22px;
        line-height: 28px;
        font-weight: 700;
        font-variant-numeric: tabular-nums;
        color: var(--tt-text);
      }
    }

    &__bad {
      color: var(--tt-danger) !important;
    }

    &__empty {
      margin: 0;
      font-size: 14px;
      color: var(--tt-text-3);

      &--padded {
        padding: 24px 16px;
      }
    }

    &__hint {
      margin: 12px 0 0;
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__alerts {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px 16px 0;
    }
  }
</style>
