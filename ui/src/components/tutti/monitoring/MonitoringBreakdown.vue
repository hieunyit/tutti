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
  import { NDataTable, type DataTableColumns } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import { useNumberFormat } from './useNumberFormat';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';

  const { t } = useI18n({ messages: i18nMonitoring });
  const fmt = useNumberFormat();
  const store = useTuttiMonitoringStore();
  const { proxies, latestSnapshots, latestByProxy } = storeToRefs(store);

  interface ProxyRow {
    appId: string;
    address: string;
    startedAt: string;
    requests: number;
    rate: number;
    e4: number;
    e5: number;
    bytesIn: number;
    bytesOut: number;
    failFast: number;
    accessDenied: number;
  }

  interface StorageRow {
    storage: string;
    requests: number;
    rate: number;
  }

  const proxyRows = computed<ProxyRow[]>(() =>
    proxies.value.flatMap((proxy) => {
      const snap = latestSnapshots.value[proxy.appId];

      if (!snap) return [];

      return [
        {
          appId: proxy.appId,
          address: proxy.address ?? '',
          startedAt: snap.startedAt,
          requests: snap.requestsTotal,
          rate: latestByProxy.value[proxy.appId]?.requests ?? 0,
          e4: snap.responses4xx,
          e5: snap.responses5xx,
          bytesIn: snap.bytesIn,
          bytesOut: snap.bytesOut,
          failFast: snap.failFast,
          accessDenied: snap.accessDenied,
        },
      ];
    }),
  );

  const storageRows = computed<StorageRow[]>(() => {
    const totals: Record<string, number> = {};
    const rates: Record<string, number> = {};

    Object.values(latestSnapshots.value).forEach((snap) => {
      Object.entries(snap.byStorage).forEach(([name, count]) => {
        totals[name] = (totals[name] ?? 0) + count;
      });
    });
    Object.values(latestByProxy.value).forEach((r) => {
      Object.entries(r.byStorage).forEach(([name, rate]) => {
        rates[name] = (rates[name] ?? 0) + rate;
      });
    });

    return Object.keys(totals)
      .sort()
      .map((storage) => ({
        storage,
        requests: totals[storage] ?? 0,
        rate: rates[storage] ?? 0,
      }));
  });

  const rateText = (v: number) =>
    `${TrafficStatsHelper.formatRate(v)}${t('rateUnit')}`;

  const num = (title: string, key: keyof ProxyRow, width = 110) => ({
    title,
    key,
    align: 'right' as const,
    width,
    render: (row: ProxyRow) => fmt(Number(row[key])),
  });

  const proxyColumns = computed<DataTableColumns<ProxyRow>>(() => [
    {
      title: t('columnProxy'),
      key: 'appId',
      minWidth: 160,
      render: (row) =>
        h('div', { class: 'monitoring-breakdown__proxy' }, [
          h('span', row.appId),
          row.address
            ? h('small', { class: 'monitoring-breakdown__muted' }, row.address)
            : null,
        ]),
    },
    {
      title: t('columnStarted'),
      key: 'startedAt',
      width: 190,
      render: (row) => formatDateTime(row.startedAt),
    },
    num(t('columnRequests'), 'requests', 130),
    {
      title: t('columnRate'),
      key: 'rate',
      align: 'right',
      width: 100,
      render: (row) => rateText(row.rate),
    },
    num(t('columnErrors4xx'), 'e4'),
    num(t('columnErrors5xx'), 'e5'),
    {
      title: t('columnBytesIn'),
      key: 'bytesIn',
      align: 'right',
      width: 110,
      render: (row) => TrafficStatsHelper.formatBytes(row.bytesIn),
    },
    {
      title: t('columnBytesOut'),
      key: 'bytesOut',
      align: 'right',
      width: 110,
      render: (row) => TrafficStatsHelper.formatBytes(row.bytesOut),
    },
    num(t('columnFailFast'), 'failFast', 100),
    num(t('columnAccessDenied'), 'accessDenied', 100),
  ]);

  const storageColumns = computed<DataTableColumns<StorageRow>>(() => [
    { title: t('columnStorage'), key: 'storage' },
    {
      title: t('columnRequests'),
      key: 'requests',
      align: 'right',
      render: (row) => fmt(row.requests),
    },
    {
      title: t('columnRate'),
      key: 'rate',
      align: 'right',
      render: (row) => rateText(row.rate),
    },
  ]);
</script>

<template>
  <div class="monitoring-breakdown">
    <CTile class="monitoring-breakdown__tile">
      <template #title>{{ t('proxiesTitle') }}</template>
      <NDataTable
        :columns="proxyColumns"
        :data="proxyRows"
        :bordered="false"
        :scroll-x="1180"
        :row-key="(row: ProxyRow) => row.appId"
      >
        <template #empty>{{ t('noProxyRows') }}</template>
      </NDataTable>
    </CTile>

    <CTile class="monitoring-breakdown__tile">
      <template #title>{{ t('storagesTitle') }}</template>
      <NDataTable
        :columns="storageColumns"
        :data="storageRows"
        :bordered="false"
        :row-key="(row: StorageRow) => row.storage"
      >
        <template #empty>{{ t('noStorageRows') }}</template>
      </NDataTable>
    </CTile>
  </div>
</template>

<style lang="scss" scoped>
  .monitoring-breakdown {
    display: grid;
    gap: 16px;

    :deep(.monitoring-breakdown__proxy) {
      display: flex;
      flex-direction: column;
    }

    :deep(.monitoring-breakdown__muted) {
      color: var(--text-color-3);
    }

    :deep(.n-data-table-td),
    :deep(.n-data-table-th) {
      font-variant-numeric: tabular-nums;
    }
  }
</style>
