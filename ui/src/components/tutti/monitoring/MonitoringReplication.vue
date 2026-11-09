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
  import { CTile } from '@clyso/clyso-ui-kit';
  import {
    NAlert,
    NDataTable,
    NProgress,
    NStatistic,
    NTag,
    type DataTableColumns,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import type { ChorusReplication } from '@/utils/types/chorus';

  const { t } = useI18n({ messages: i18nMonitoring });
  const store = useTuttiMonitoringStore();
  const { replications, replicationsError, latestSnapshots, current } =
    storeToRefs(store);

  const num = (v: string | number | undefined) =>
    TrafficStatsHelper.toNumber(v);

  const sync = computed(() => {
    const snaps = Object.values(latestSnapshots.value);
    const sum = (
      key: 'syncQuorumReached' | 'syncBelowQuorum' | 'syncSkipped',
    ) => snaps.reduce((total, s) => total + s[key], 0);

    return {
      reached: sum('syncQuorumReached'),
      below: sum('syncBelowQuorum'),
      skipped: sum('syncSkipped'),
    };
  });

  const laggingCount = computed(
    () => replications.value.filter((r) => store.isLagging(r)).length,
  );

  function label(replication: ChorusReplication): string {
    const { user, fromStorage, toStorage, fromBucket } = replication.id;

    return `${user}: ${fromStorage} → ${toStorage}${fromBucket ? ` (${fromBucket})` : ''}`;
  }

  const columns = computed<DataTableColumns<ChorusReplication>>(() => [
    { title: t('columnReplication'), key: 'label', render: label },
    {
      title: t('columnEvents'),
      key: 'events',
      render: (r) => String(num(r.events)),
    },
    {
      title: t('columnEventsDone'),
      key: 'eventsDone',
      render: (r) => String(num(r.eventsDone)),
    },
    {
      title: t('columnLag'),
      key: 'eventLag',
      render: (r) => {
        const lag = num(r.eventLag);

        return h(
          NTag,
          {
            size: 'small',
            type:
              lag === 0
                ? 'success'
                : store.isLagging(r)
                  ? 'warning'
                  : 'default',
          },
          () => String(lag),
        );
      },
    },
    {
      title: t('columnInitSync'),
      key: 'init',
      render: (r) => {
        if (r.isInitDone) return t('initDone');

        const listed = num(r.initObjListed);
        const done = num(r.initObjDone);

        return h('div', { style: 'min-width: 140px' }, [
          h(NProgress, {
            type: 'line',
            percentage: listed > 0 ? Math.round((done / listed) * 100) : 0,
            indicatorPlacement: 'inside',
            height: 14,
          }),
          h('small', `${done} / ${listed}`),
        ]);
      },
    },
    {
      title: t('columnState'),
      key: 'isPaused',
      render: (r) =>
        h(NTag, { size: 'small', type: r.isPaused ? 'warning' : 'info' }, () =>
          r.isPaused ? t('statePaused') : t('stateActive'),
        ),
    },
  ]);
</script>

<template>
  <div class="monitoring-replication">
    <CTile class="monitoring-replication__tile">
      <template #title>{{ t('syncTitle') }}</template>
      <div class="monitoring-replication__sync">
        <NStatistic
          :label="t('syncQuorumReached')"
          :value="sync.reached"
        />
        <NStatistic
          :label="t('syncBelowQuorum')"
          :value="sync.below"
        />
        <NStatistic
          :label="t('syncSkipped')"
          :value="sync.skipped"
        />
        <NStatistic
          v-if="current"
          :label="`${t('syncBelowQuorum')}${t('rateUnit')}`"
          :value="TrafficStatsHelper.formatRate(current.syncBelowQuorum)"
        />
      </div>
      <p class="monitoring-replication__hint">{{ t('syncHint') }}</p>
    </CTile>

    <CTile class="monitoring-replication__tile">
      <template #title>{{ t('backlogTitle') }}</template>
      <NAlert
        v-if="replicationsError"
        type="error"
        class="monitoring-replication__alert"
      >
        {{ t('backlogError') }}
      </NAlert>
      <NAlert
        v-if="laggingCount > 0"
        type="warning"
        :title="t('lagWarning')"
        class="monitoring-replication__alert"
      >
        {{ t('lagWarningText', { count: laggingCount }) }}
      </NAlert>
      <NDataTable
        :columns="columns"
        :data="replications"
        :bordered="false"
        :scroll-x="800"
        :row-key="(row: ChorusReplication) => row.idStr"
      >
        <template #empty>{{ t('backlogEmpty') }}</template>
      </NDataTable>
    </CTile>
  </div>
</template>

<style lang="scss" scoped>
  .monitoring-replication {
    display: grid;
    gap: 16px;

    &__sync {
      display: flex;
      flex-wrap: wrap;
      gap: 32px;
    }

    &__hint {
      margin: 12px 0 0;
      font-size: 12px;
      opacity: 0.7;
    }

    &__alert {
      margin-bottom: 12px;
    }
  }
</style>
