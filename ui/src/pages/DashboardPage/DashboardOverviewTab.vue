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
  import { storeToRefs } from 'pinia';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import DashboardAttention from '@/components/tutti/dashboard/DashboardAttention.vue';
  import DashboardEndpoint from '@/components/tutti/dashboard/DashboardEndpoint.vue';
  import DashboardKpis from '@/components/tutti/dashboard/DashboardKpis.vue';
  import DashboardReplications from '@/components/tutti/dashboard/DashboardReplications.vue';
  import DashboardStorages from '@/components/tutti/dashboard/DashboardStorages.vue';
  import ProxyUptimeWidget from '@/components/tutti/home/ProxyUptimeWidget/ProxyUptimeWidget.vue';
  import WorkerUptimeWidget from '@/components/tutti/home/WorkerUptimeWidget/WorkerUptimeWidget.vue';
  import { Callout } from '@/components/kit';
  import { useTuttiDashboardStore } from '@/stores/tuttiDashboardStore';
  import { useNow } from '@/utils/composables/useNow';
  import { HAS_PROMETHEUS } from '@/utils/constants/env';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';

  const { t } = useI18n({ messages: i18nDashboard });
  const {
    storages,
    replications,
    proxies,
    events,
    policies,
    totalRates,
    failedSources,
  } = storeToRefs(useTuttiDashboardStore());
  const now = useNow(5000);

  // only needed by the optional Prometheus uptime widgets
  const loadingState = ref(0);
  const isLoadingFinished = ref(false);
  const isUptimeLoading = computed(
    () => !isLoadingFinished.value || loadingState.value > 0,
  );

  function setLoadingState(value: boolean) {
    loadingState.value += value ? 1 : -1;

    if (loadingState.value === 0) isLoadingFinished.value = true;
  }

  const storageRows = computed(() =>
    storages.value
      ? DashboardHelper.getStorageHealth(
          storages.value,
          proxies.value,
          now.value,
        )
      : null,
  );
  const breakdown = computed(() =>
    replications.value
      ? DashboardHelper.getReplicationBreakdown(replications.value)
      : null,
  );
  const reporting = computed(() =>
    proxies.value
      ? DashboardHelper.getProxyReporting(proxies.value, now.value)
      : null,
  );
  const traffic = computed(() =>
    DashboardHelper.getTrafficSummary(totalRates.value),
  );
  const storageRates = computed(() => traffic.value?.byStorage ?? null);
  const p95 = computed(() =>
    traffic.value?.p95Ms == null
      ? ''
      : TrafficStatsHelper.formatLatencyMs(traffic.value.p95Ms),
  );
  const attention = computed(() =>
    DashboardHelper.getAttentionItems(
      {
        storages: storages.value,
        replications: replications.value,
        proxies: proxies.value,
        events: events.value,
        policies: policies.value,
      },
      now.value,
    ),
  );
  const partialErrorText = computed(() =>
    t('partialError', {
      sources: failedSources.value
        .map((key) => t(`source${key.charAt(0).toUpperCase()}${key.slice(1)}`))
        .join(', '),
    }),
  );
  const hasAllData = computed(
    () =>
      failedSources.value.length === 0 &&
      !!replications.value &&
      (storageRows.value?.length ?? 0) > 0 &&
      storageRows.value?.every((storage) => storage.state === 'healthy') &&
      (reporting.value?.reporting ?? 0) > 0,
  );
</script>

<template>
  <div class="dashboard-overview">
    <Callout
      v-if="failedSources.length > 0"
      tone="warning"
    >
      {{ partialErrorText }}
    </Callout>

    <DashboardAttention
      v-if="attention.length > 0"
      :items="attention"
    />
    <div
      v-else-if="hasAllData"
      class="dashboard-overview__ok"
      role="status"
    >
      <span
        class="dashboard-overview__ok-dot"
        aria-hidden="true"
      />
      {{ t('allGood') }}
    </div>

    <DashboardKpis
      :storages="storageRows"
      :replications="breakdown"
      :proxies="reporting"
      :traffic="traffic"
    />

    <div class="dashboard-overview__columns">
      <DashboardStorages
        :storages="storageRows"
        :rates="storageRates"
        :p95="p95"
      />
      <DashboardReplications
        v-if="breakdown && replications"
        :breakdown="breakdown"
        :replications="replications"
      />
    </div>

    <DashboardEndpoint />

    <div
      v-if="HAS_PROMETHEUS"
      class="dashboard-overview__uptime"
    >
      <ProxyUptimeWidget
        :is-page-loading="isUptimeLoading"
        :is-enabled="HAS_PROMETHEUS"
        @loading="setLoadingState"
      />
      <WorkerUptimeWidget
        :is-page-loading="isUptimeLoading"
        :is-enabled="HAS_PROMETHEUS"
        @loading="setLoadingState"
      />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .dashboard-overview {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);

    &__ok {
      display: flex;
      align-items: center;
      gap: var(--tt-space-2);
      padding: var(--tt-space-3) var(--tt-card-padding);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
      font-size: var(--tt-text-md);
      color: var(--tt-text);
    }

    &__ok-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background-color: var(--tt-success-solid);
    }

    &__columns {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(min(420px, 100%), 1fr));
      gap: var(--tt-space-4);
      align-items: start;
    }

    &__uptime {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(min(320px, 100%), 1fr));
      gap: var(--tt-space-4);

      > :deep(*) {
        grid-column: auto !important;
      }
    }
  }
</style>
