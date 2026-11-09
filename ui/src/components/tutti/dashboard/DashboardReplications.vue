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
  import { CButton, CTile } from '@clyso/clyso-ui-kit';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useReplicationsWidget } from '@/components/chorus/home/ReplicationsWidget/useReplicationsWidget';
  import {
    DashboardHelper,
    type ReplicationBreakdown,
  } from '@/utils/helpers/DashboardHelper';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    breakdown: ReplicationBreakdown;
  }>();

  const { t } = useI18n({ messages: i18nDashboard });
  const {
    goToReplicationsPage,
    showUpToDateReplications,
    showInProgressReplications,
    showBehindReplications,
    showPausedReplications,
  } = useReplicationsWidget();

  const rows = computed(() => {
    const b = props.breakdown;

    return [
      {
        key: 'upToDate',
        label: t('breakdownUpToDate'),
        count: b.upToDate,
        color: 'var(--success-color)',
        open: showUpToDateReplications,
      },
      {
        key: 'initial',
        label: t('breakdownInitial'),
        count: b.initial,
        color: 'var(--info-color)',
        open: showInProgressReplications,
      },
      {
        key: 'behind',
        label: t('breakdownBehind'),
        count: b.behind,
        color: 'var(--warning-color)',
        open: showBehindReplications,
      },
      {
        key: 'paused',
        label: t('breakdownPaused'),
        count: b.paused,
        color: 'var(--text-color-3)',
        open: showPausedReplications,
      },
    ].map((row) => ({
      ...row,
      share: DashboardHelper.percent(row.count, b.total),
    }));
  });

  const ariaLabel = computed(
    () =>
      `${t('breakdownLabel')}: ${rows.value
        .map((row) => `${row.label} ${row.count}`)
        .join(', ')}`,
  );
</script>

<template>
  <CTile class="dashboard-replications">
    <template #title>{{ t('replicationsTitle') }}</template>
    <template #header-extra>
      <RouterLink
        class="dashboard-replications__link"
        :to="{ name: RouteName.CHORUS_REPLICATION }"
      >
        {{ t('replicationsAll') }} →
      </RouterLink>
    </template>

    <div
      v-if="breakdown.total === 0"
      class="dashboard-replications__empty"
    >
      <p class="dashboard-replications__empty-title">
        {{ t('replicationsEmptyTitle') }}
      </p>
      <p class="dashboard-replications__empty-text">
        {{ t('replicationsEmptyText') }}
      </p>
      <WriteGate>
        <CButton
          type="primary"
          size="small"
          @click="$router.push({ name: RouteName.CHORUS_ADD_REPLICATION })"
        >
          {{ t('replicationsAdd') }}
        </CButton>
      </WriteGate>
    </div>

    <template v-else>
      <div
        class="dashboard-replications__bar"
        role="img"
        :aria-label="ariaLabel"
      >
        <template
          v-for="row in rows"
          :key="row.key"
        >
          <span
            v-if="row.count > 0"
            class="dashboard-replications__segment"
            :style="{ flexGrow: row.count, backgroundColor: row.color }"
            :title="`${row.label}: ${row.count}`"
          />
        </template>
      </div>

      <table class="dashboard-replications__table">
        <thead>
          <tr>
            <th scope="col">{{ t('columnStatus') }}</th>
            <th
              scope="col"
              class="dashboard-replications__num"
            >
              {{ t('columnCount') }}
            </th>
            <th
              scope="col"
              class="dashboard-replications__num"
            >
              {{ t('columnShare') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="row.key"
            class="dashboard-replications__row"
            tabindex="0"
            @click="row.open()"
            @keydown.enter="row.open()"
          >
            <td>
              <span
                class="dashboard-replications__swatch"
                :style="{ backgroundColor: row.color }"
              />
              {{ row.label }}
            </td>
            <td class="dashboard-replications__num">{{ row.count }}</td>
            <td class="dashboard-replications__num">{{ row.share }} %</td>
          </tr>
        </tbody>
      </table>

      <button
        type="button"
        class="dashboard-replications__total"
        @click="goToReplicationsPage"
      >
        {{ t('breakdownTotal', { n: breakdown.total }) }}
      </button>
    </template>
  </CTile>
</template>

<style lang="scss" scoped>
  .dashboard-replications {
    &__link {
      font-size: 13px;
      color: var(--primary-color);
      text-decoration: none;

      &:hover,
      &:focus-visible {
        text-decoration: underline;
      }
    }

    &__empty {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 8px;
    }

    &__empty-title {
      margin: 0;
      font-weight: 600;
      color: var(--text-color-1);
    }

    &__empty-text {
      margin: 0;
      font-size: 13px;
      color: var(--text-color-3);
    }

    &__bar {
      display: flex;
      gap: 2px;
      height: 12px;
      margin-bottom: 12px;
      border-radius: 6px;
      overflow: hidden;
      background-color: var(--border-color);
    }

    &__segment {
      flex-basis: 0;
      min-width: 6px;
    }

    &__table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;

      th {
        padding: 4px 8px;
        font-size: 12px;
        font-weight: 500;
        text-align: left;
        color: var(--text-color-3);
      }

      td {
        padding: 8px;
        color: var(--text-color-1);
        border-top: 1px solid var(--border-color);
      }
    }

    &__row {
      cursor: pointer;

      &:hover,
      &:focus-visible {
        outline: none;
        background-color: color-mix(
          in srgb,
          var(--primary-color) 8%,
          transparent
        );
      }
    }

    &__num {
      width: 72px;
      text-align: right !important;
      font-variant-numeric: tabular-nums;
    }

    &__swatch {
      display: inline-block;
      width: 10px;
      height: 10px;
      margin-right: 8px;
      border-radius: 3px;
    }

    &__total {
      margin-top: 8px;
      padding: 0;
      border: 0;
      background: none;
      font: inherit;
      font-size: 13px;
      cursor: pointer;
      color: var(--text-color-3);

      &:hover {
        color: var(--primary-color);
      }
    }
  }
</style>
