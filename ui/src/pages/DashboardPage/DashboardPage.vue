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
  import { NButton } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import DashboardMetricsTab from './DashboardMetricsTab.vue';
  import DashboardOverviewTab from './DashboardOverviewTab.vue';
  import i18nDashboard from './i18nDashboard';
  import { useDashboardData } from './useDashboardData';
  import { PageShell, useQueryTab } from '@/components/kit';

  const { t } = useI18n({ messages: i18nDashboard });
  const data = useDashboardData();
  const activeTab = useQueryTab('tab', 'overview');

  const tabs = computed(() => [
    { key: 'overview', label: t('tabOverview') },
    { key: 'metrics', label: t('tabMetrics') },
  ]);
  const isMetrics = computed(() => activeTab.value === 'metrics');
  const shellState = computed(() =>
    isMetrics.value ? 'ready' : data.state.value,
  );
  const updatedText = computed(() =>
    data.updatedAt.value === null
      ? t('notUpdated')
      : t('updatedAt', {
          time: data.updatedAt.value.toLocaleTimeString([], {
            hour: '2-digit',
            minute: '2-digit',
          }),
        }),
  );
</script>

<template>
  <PageShell
    :title="t('title')"
    :description="t('description')"
    :state="shellState"
    :tabs="tabs"
    :active-tab="activeTab"
    :error-text="t('errorText')"
    @update:active-tab="activeTab = $event"
    @retry="data.refresh()"
  >
    <template
      v-if="!isMetrics"
      #actions
    >
      <span
        class="dashboard-page__updated"
        role="status"
        >{{ updatedText }}</span
      >
      <NButton @click="data.refresh()">{{ t('refresh') }}</NButton>
    </template>

    <DashboardMetricsTab v-if="isMetrics" />
    <DashboardOverviewTab v-else />
  </PageShell>
</template>

<style lang="scss" scoped>
  .dashboard-page__updated {
    align-self: center;
    margin-right: var(--tt-space-3);
    font-size: var(--tt-text-sm);
    color: var(--tt-text-2);
  }
</style>
