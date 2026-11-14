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
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import type { StatusVariant } from '@/components/ui/status';
  import type {
    StorageHealthRow,
    StorageHealthState,
  } from '@/utils/helpers/DashboardHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import { RouteName } from '@/utils/types/router';

  defineProps<{
    storages: StorageHealthRow[] | null;
    /** requests/s per storage name, null while no rate is known */
    rates: Record<string, number> | null;
    /** formatted overall p95 latency, shown in the header */
    p95?: string;
  }>();

  const { t } = useI18n({ messages: i18nDashboard });

  const VARIANT: Record<StorageHealthState, StatusVariant> = {
    healthy: 'success',
    unavailable: 'danger',
    unknown: 'neutral',
  };

  const STATE_KEY: Record<StorageHealthState, string> = {
    healthy: 'stateHealthy',
    unavailable: 'stateUnavailable',
    unknown: 'stateUnknown',
  };
</script>

<template>
  <SectionCard
    class="dashboard-storages"
    :title="t('storagesTitle')"
    :meta="p95 ? t('storagesP95', { value: p95 }) : undefined"
    :link-label="t('storagesViewAll')"
    :link-to="{ name: RouteName.TUTTI_STORAGES }"
    flush
  >
    <p
      v-if="storages === null"
      class="dashboard-storages__note"
    >
      {{ t('storagesUnavailable') }}
    </p>

    <p
      v-else-if="storages.length === 0"
      class="dashboard-storages__note"
    >
      {{ t('storagesEmpty') }}
    </p>

    <table
      v-else
      class="tt-table"
    >
      <thead>
        <tr>
          <th scope="col">{{ t('columnName') }}</th>
          <th scope="col">{{ t('columnProvider') }}</th>
          <th scope="col">{{ t('columnStatus') }}</th>
          <th
            scope="col"
            class="tt-num"
          >
            {{ t('columnReqPerSecond') }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="storage in storages"
          :key="storage.name"
        >
          <td class="dashboard-storages__name">
            <RouterLink
              :to="{
                name: RouteName.TUTTI_STORAGE_DETAILS,
                params: { storageName: storage.name },
              }"
            >
              {{ storage.name }}
            </RouterLink>
          </td>
          <td>{{ storage.provider || 'S3' }}</td>
          <td>
            <StatusTag
              :variant="VARIANT[storage.state]"
              :label="t(STATE_KEY[storage.state])"
            />
          </td>
          <td class="tt-num">
            {{
              rates && storage.name in rates
                ? TrafficStatsHelper.formatRate(rates[storage.name] ?? 0)
                : t('kpiNoData')
            }}
          </td>
        </tr>
      </tbody>
    </table>
  </SectionCard>
</template>

<style lang="scss" scoped>
  .dashboard-storages {
    &__note {
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
  }
</style>
