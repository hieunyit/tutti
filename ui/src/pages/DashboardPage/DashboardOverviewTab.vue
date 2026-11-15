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

    <DashboardKpis
      :storages="storageRows"
      :replications="breakdown"
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
