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
  import i18nDashboard from './i18nDashboard';
  import DashboardReplicationSummary from './DashboardReplicationSummary.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import type { StatusVariant } from '@/components/ui/status';
  import {
    DashboardHelper,
    type ReplicationBreakdown,
    type ReplicationState,
  } from '@/utils/helpers/DashboardHelper';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import { IconName } from '@/utils/types/icon';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    breakdown: ReplicationBreakdown;
    replications: TuttiReplication[];
  }>();

  const { t } = useI18n({ messages: i18nDashboard });
  const number = useNumberFormat();

  const VARIANT: Record<ReplicationState, StatusVariant> = {
    upToDate: 'success',
    initial: 'info',
    behind: 'warning',
    paused: 'warning',
  };

  const rows = computed(() =>
    DashboardHelper.summaryReplications(props.replications).map(
      (replication) => ({
        id: replication.idStr,
        name: DashboardHelper.replicationLabel(replication),
        state: DashboardHelper.replicationState(replication),
        lag: DashboardHelper.eventLag(replication),
      }),
    ),
  );
</script>

<template>
  <SectionCard
    class="dashboard-replications"
    :title="t('replicationsTitle')"
    :link-label="breakdown.total > 0 ? t('storagesViewAll') : undefined"
    :link-to="
      breakdown.total > 0 ? { name: RouteName.TUTTI_REPLICATION } : undefined
    "
    flush
  >
    <EmptyState
      v-if="breakdown.total === 0"
      :icon="IconName.BASE_SWAP_HORIZONTAL"
      :title="t('replicationsEmptyTitle')"
      :text="t('replicationsEmptyText')"
    >
      <WriteGate>
        <NButton
          type="primary"
          @click="$router.push({ name: RouteName.TUTTI_ADD_REPLICATION })"
        >
          {{ t('replicationsAdd') }}
        </NButton>
      </WriteGate>
    </EmptyState>

    <template v-else>
      <DashboardReplicationSummary :breakdown="breakdown" />

      <div class="dashboard-replications__table tt-table-wrap">
        <table class="tt-table">
          <thead>
            <tr>
              <th scope="col">{{ t('columnReplication') }}</th>
              <th scope="col">{{ t('columnStatus') }}</th>
              <th
                scope="col"
                class="tt-num"
              >
                {{ t('columnLag') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in rows"
              :key="row.id"
            >
              <td>{{ row.name }}</td>
              <td>
                <StatusTag
                  :variant="VARIANT[row.state]"
                  :label="t(`replicationState_${row.state}`)"
                />
              </td>
              <td class="tt-num">{{ number(row.lag) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </SectionCard>
</template>

<style lang="scss" scoped>
  .dashboard-replications {
    &__table {
      border-top: 1px solid var(--tt-border-subtle);
    }
  }
</style>
