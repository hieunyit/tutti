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
  import { CButton, CSkeleton } from '@clyso/clyso-ui-kit';
  import { NAlert } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import DashboardAttention from '@/components/tutti/dashboard/DashboardAttention.vue';
  import DashboardEndpoint from '@/components/tutti/dashboard/DashboardEndpoint.vue';
  import DashboardKpis from '@/components/tutti/dashboard/DashboardKpis.vue';
  import DashboardReplications from '@/components/tutti/dashboard/DashboardReplications.vue';
  import DashboardStorages from '@/components/tutti/dashboard/DashboardStorages.vue';
  import i18nDashboard from '@/components/tutti/dashboard/i18nDashboard';
  import ChorusListError from '@/components/chorus/common/ChorusListError/ChorusListError.vue';
  import ProxyUptimeWidget from '@/components/chorus/home/ProxyUptimeWidget/ProxyUptimeWidget.vue';
  import WorkerUptimeWidget from '@/components/chorus/home/WorkerUptimeWidget/WorkerUptimeWidget.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import { useTuttiDashboardStore } from '@/stores/tuttiDashboardStore';
  import { useNow } from '@/utils/composables/useNow';
  import { HAS_PROMETHEUS } from '@/utils/constants/env';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';

  const { t } = useI18n({ messages: i18nDashboard });
  const store = useTuttiDashboardStore();
  const {
    isLoading,
    isRefreshing,
    lastUpdated,
    storages,
    replications,
    proxies,
    events,
    windows,
    failedSources,
    hasFatalError,
  } = storeToRefs(store);
  const now = useNow(5000);

  onBeforeMount(() => store.startPolling());
  onBeforeUnmount(() => store.stopPolling());

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

  const updatedText = computed(() =>
    lastUpdated.value === null
      ? t('notUpdated')
      : t('updated', {
          time: formatRelative(lastUpdated.value, undefined, now.value),
        }),
  );

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
    Object.keys(windows.value).length > 0
      ? DashboardHelper.getTrafficSummary(windows.value, now.value)
      : null,
  );
  const attention = computed(() =>
    DashboardHelper.getAttentionItems(
      {
        storages: storages.value,
        replications: replications.value,
        proxies: proxies.value,
        events: events.value,
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
  <div class="chorus-home-page">
    <PageHeader
      :title="t('title')"
      :description="t('description')"
    >
      <template #actions>
        <span
          class="chorus-home-page__updated"
          aria-live="polite"
        >
          {{ updatedText }}
        </span>
        <CButton
          :loading="isRefreshing"
          @click="store.refresh()"
        >
          {{ t('refresh') }}
        </CButton>
      </template>
    </PageHeader>

    <ChorusListError
      v-if="hasFatalError"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-text="t('retry')"
      @retry="store.refresh()"
    />

    <div
      v-else-if="isLoading"
      class="chorus-home-page__sections"
    >
      <div class="chorus-home-page__skeleton-kpis">
        <CSkeleton
          v-for="n in 6"
          :key="n"
          :height="104"
          :border-radius="8"
        />
      </div>
      <div class="chorus-home-page__columns">
        <CSkeleton
          :height="240"
          :border-radius="8"
        />
        <CSkeleton
          :height="240"
          :border-radius="8"
        />
      </div>
    </div>

    <div
      v-else
      class="chorus-home-page__sections"
    >
      <NAlert
        v-if="failedSources.length > 0"
        type="warning"
      >
        {{ partialErrorText }}
      </NAlert>

      <DashboardAttention
        v-if="attention.length > 0"
        :items="attention"
      />
      <div
        v-else-if="hasAllData"
        class="chorus-home-page__ok"
        role="status"
      >
        <span
          class="chorus-home-page__ok-dot"
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

      <div class="chorus-home-page__columns">
        <DashboardStorages :storages="storageRows" />

        <div class="chorus-home-page__side">
          <DashboardReplications
            v-if="breakdown"
            :breakdown="breakdown"
          />
          <DashboardEndpoint />
        </div>
      </div>

      <div
        v-if="HAS_PROMETHEUS"
        class="chorus-home-page__uptime"
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
  </div>
</template>

<style lang="scss" scoped>
  .chorus-home-page {
    max-width: 1800px;

    &__updated {
      font-size: 13px;
      color: var(--text-color-3);
    }

    &__sections {
      display: flex;
      flex-direction: column;
      gap: 16px;
    }

    &__ok {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 10px 16px;
      border: 1px solid var(--border-color);
      border-radius: 8px;
      background-color: var(--card-color);
      font-size: 14px;
      color: var(--text-color-1);
    }

    &__ok-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background-color: var(--success-color);
    }

    &__skeleton-kpis {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
      gap: 12px;
    }

    &__columns {
      display: grid;
      grid-template-columns: minmax(0, 7fr) minmax(0, 5fr);
      gap: 16px;
      align-items: start;

      @media screen and (max-width: 1100px) {
        grid-template-columns: minmax(0, 1fr);
      }
    }

    &__side {
      display: flex;
      flex-direction: column;
      gap: 16px;
      min-width: 0;
    }

    &__uptime {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 16px;

      > :deep(*) {
        grid-column: auto !important;
      }

      @media screen and (max-width: 500px) {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  }
</style>
