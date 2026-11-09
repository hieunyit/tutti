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
  import SectionCard from '@/components/ui/SectionCard.vue';
  import { EMPTY_VALUE } from '@/components/ui/status';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { useNow } from '@/utils/composables/useNow';
  import {
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nMonitoring });
  const fmt = useNumberFormat();
  const store = useTuttiMonitoringStore();
  const { proxies, totalRates } = storeToRefs(store);
  const now = useNow(1000);

  const rate = (v: number) => TrafficStatsHelper.formatRate(v);

  const storageRows = computed(() => {
    const totals: Record<string, number> = {};

    proxies.value.forEach((proxy) => {
      Object.entries(proxy.requestsByStorage).forEach(([name, count]) => {
        totals[name] = (totals[name] ?? 0) + count;
      });
    });

    const names = new Set([
      ...Object.keys(totals),
      ...Object.keys(totalRates.value?.byStorage ?? {}),
    ]);

    return [...names].sort().map((name) => ({
      name,
      rate: totalRates.value?.byStorage[name] ?? 0,
      total: totals[name] ?? 0,
    }));
  });

  const proxyRows = computed(() =>
    proxies.value.map((proxy) => ({
      appId: proxy.appId,
      address: proxy.address ?? '',
      startedAt: proxy.startedAt,
      updatedAt: proxy.updatedAt,
      rates: proxy.rates,
    })),
  );
</script>

<template>
  <div class="monitoring-breakdown">
    <SectionCard
      :title="t('storagesTitle')"
      flush
    >
      <p
        v-if="storageRows.length === 0"
        class="monitoring-breakdown__empty"
      >
        {{ t('noStorageRows') }}
      </p>
      <table
        v-else
        class="tt-table"
      >
        <caption class="tt-sr-only">
          {{
            t('storagesTitle')
          }}
        </caption>
        <thead>
          <tr>
            <th scope="col">{{ t('columnStorage') }}</th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnRate') }}
            </th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnRequests') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in storageRows"
            :key="row.name"
          >
            <td class="monitoring-breakdown__name">
              <RouterLink
                :to="{
                  name: RouteName.CHORUS_STORAGE_DETAILS,
                  params: { storageName: row.name },
                }"
              >
                {{ row.name }}
              </RouterLink>
            </td>
            <td class="tt-num">{{ rate(row.rate) }}</td>
            <td class="tt-num">{{ fmt(row.total) }}</td>
          </tr>
        </tbody>
      </table>
    </SectionCard>

    <SectionCard
      :title="t('proxiesTitle')"
      flush
    >
      <p
        v-if="proxyRows.length === 0"
        class="monitoring-breakdown__empty"
      >
        {{ t('noProxyRows') }}
      </p>
      <table
        v-else
        class="tt-table"
      >
        <caption class="tt-sr-only">
          {{
            t('proxiesTitle')
          }}
        </caption>
        <thead>
          <tr>
            <th scope="col">{{ t('columnProxy') }}</th>
            <th scope="col">{{ t('columnStarted') }}</th>
            <th scope="col">{{ t('columnLastReport') }}</th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnRate') }}
            </th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnP95') }}
            </th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnErrors5xx') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in proxyRows"
            :key="row.appId"
          >
            <td>
              <div class="monitoring-breakdown__proxy">
                <span class="tt-mono">{{ row.address || row.appId }}</span>
                <small v-if="row.address">{{ row.appId }}</small>
              </div>
            </td>
            <td>{{ formatDateTime(row.startedAt) }}</td>
            <td :title="formatDateTime(row.updatedAt)">
              {{ formatRelative(row.updatedAt, undefined, now) }}
            </td>
            <td class="tt-num">
              {{ row.rates ? rate(row.rates.requests) : EMPTY_VALUE }}
            </td>
            <td class="tt-num">
              {{
                row.rates && row.rates.requests > 0
                  ? TrafficStatsHelper.formatLatencyMs(row.rates.latencyP95)
                  : EMPTY_VALUE
              }}
            </td>
            <td class="tt-num">
              {{ row.rates ? rate(row.rates.status5xx) : EMPTY_VALUE }}
            </td>
          </tr>
        </tbody>
      </table>
    </SectionCard>
  </div>
</template>

<style lang="scss" scoped>
  .monitoring-breakdown {
    display: grid;
    gap: 16px;

    &__empty {
      margin: 0;
      padding: 24px 16px;
      font-size: 14px;
      color: var(--tt-text-3);
    }

    &__name a {
      font-weight: 600;
      color: var(--tt-text);
      text-decoration: none;

      &:hover {
        color: var(--tt-accent);
        text-decoration: underline;
      }
    }

    &__proxy {
      display: flex;
      flex-direction: column;

      small {
        font-size: 12px;
        color: var(--tt-text-3);
      }
    }
  }
</style>
