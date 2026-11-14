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
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useReplicationsWidget } from '@/components/tutti/home/ReplicationsWidget/useReplicationsWidget';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import type { StatusVariant } from '@/components/ui/status';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import {
    DashboardHelper,
    type ReplicationBreakdown,
    type ReplicationState,
  } from '@/utils/helpers/DashboardHelper';
  import { IconName } from '@/utils/types/icon';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    breakdown: ReplicationBreakdown;
    replications: TuttiReplication[];
  }>();

  const { t } = useI18n({ messages: i18nDashboard });
  const number = useNumberFormat();
  const {
    showUpToDateReplications,
    showInProgressReplications,
    showBehindReplications,
    showPausedReplications,
  } = useReplicationsWidget();

  const VARIANT: Record<ReplicationState, StatusVariant> = {
    upToDate: 'success',
    initial: 'info',
    behind: 'warning',
    paused: 'warning',
  };

  const counts = computed(() => {
    const b = props.breakdown;

    return [
      {
        key: 'upToDate',
        label: t('breakdownUpToDate'),
        count: b.upToDate,
        color: 'var(--tt-success-solid)',
        open: showUpToDateReplications,
      },
      {
        key: 'initial',
        label: t('breakdownInitial'),
        count: b.initial,
        color: 'var(--tt-info-solid)',
        open: showInProgressReplications,
      },
      {
        key: 'behind',
        label: t('breakdownBehind'),
        count: b.behind,
        color:
          'color-mix(in srgb, var(--tt-warning-solid) 55%, var(--tt-surface))',
        open: showBehindReplications,
        tone: 'warning',
      },
      {
        key: 'paused',
        label: t('breakdownPaused'),
        count: b.paused,
        color: 'var(--tt-warning-solid)',
        open: showPausedReplications,
        tone: 'warning',
      },
    ];
  });

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

  const ariaLabel = computed(
    () =>
      `${t('breakdownLabel')}: ${counts.value
        .map((row) => `${row.label} ${row.count}`)
        .join(', ')}`,
  );
</script>

<template>
  <SectionCard
    class="dashboard-replications"
    :title="t('replicationsTitle')"
    :link-label="t('storagesViewAll')"
    :link-to="{ name: RouteName.TUTTI_REPLICATION }"
    flush
  >
    <EmptyState
      v-if="breakdown.total === 0"
      :icon="IconName.BASE_SWAP_HORIZONTAL"
      :title="t('replicationsEmptyTitle')"
      :text="t('replicationsEmptyText')"
    >
      <WriteGate>
        <button
          type="button"
          class="dashboard-replications__add"
          @click="$router.push({ name: RouteName.TUTTI_ADD_REPLICATION })"
        >
          {{ t('replicationsAdd') }}
        </button>
      </WriteGate>
    </EmptyState>

    <template v-else>
      <div class="dashboard-replications__summary">
        <div
          class="dashboard-replications__bar"
          role="img"
          :aria-label="ariaLabel"
        >
          <template
            v-for="row in counts"
            :key="row.key"
          >
            <span
              v-if="row.count > 0"
              class="dashboard-replications__segment"
              :style="{ flexGrow: row.count, backgroundColor: row.color }"
            />
          </template>
        </div>

        <ul class="dashboard-replications__counts">
          <li
            v-for="row in counts"
            :key="row.key"
          >
            <button
              type="button"
              class="dashboard-replications__count"
              @click="row.open()"
            >
              <span class="dashboard-replications__count-label">
                <span
                  class="dashboard-replications__swatch"
                  aria-hidden="true"
                  :style="{ backgroundColor: row.color }"
                />
                {{ row.label }}
              </span>
              <strong
                class="dashboard-replications__count-value"
                :class="{
                  'dashboard-replications__count-value--warning':
                    row.tone === 'warning' && row.count > 0,
                }"
                >{{ number(row.count) }}</strong
              >
            </button>
          </li>
        </ul>
      </div>

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
    &__summary {
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding: var(--tt-card-padding);
    }

    &__bar {
      display: flex;
      height: 10px;
      overflow: hidden;
      border-radius: var(--tt-radius-pill);
      background-color: var(--tt-border-subtle);
    }

    &__segment {
      flex-basis: 0;
      min-width: 6px;
    }

    &__counts {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
      margin: 0;
      padding: 0;
      list-style: none;

      @media screen and (max-width: 479px) {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }

    &__count {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 2px;
      width: 100%;
      padding: 0;
      border: 0;
      background: transparent;
      font: inherit;
      font-size: 13px;
      color: var(--tt-text-2);
      text-align: left;
      cursor: pointer;

      &:hover .dashboard-replications__count-value {
        text-decoration: underline;
      }
    }

    &__count-label {
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }

    &__swatch {
      width: 8px;
      height: 8px;
      border-radius: 50%;
    }

    &__count-value {
      font-size: 18px;
      font-variant-numeric: tabular-nums;
      color: var(--tt-text);

      &--warning {
        color: var(--tt-warning);
      }
    }

    &__table {
      border-top: 1px solid var(--tt-border-subtle);
    }

    &__add {
      height: 36px;
      padding: 0 14px;
      border: 1px solid var(--tt-accent-solid);
      border-radius: var(--tt-radius-control);
      background-color: var(--tt-accent-solid);
      font: inherit;
      font-size: 14px;
      font-weight: 600;
      color: var(--tt-on-accent);
      cursor: pointer;

      &:hover {
        background-color: var(--tt-accent-solid-hover);
      }
    }
  }
</style>
