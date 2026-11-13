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
  import type { DataTableColumns } from 'naive-ui';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { DataTable, EMPTY_VALUE, StatusBadge } from '@/components/kit';
  import i18nProtection from '@/components/tutti/protection/i18nProtectionUi';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionGroup,
    ProtectionGroupEvent,
  } from '@/utils/types/protection';

  const props = defineProps<{ group: ProtectionGroup }>();

  const { t } = useI18n({ messages: i18nProtection });

  const steps = computed(() => ProtectionGroupHelper.switchSteps(props.group));
  const isSwitchActive = computed(() =>
    steps.value.some((step) => step.status !== 'pending'),
  );
  const rows = computed(() =>
    ProtectionGroupHelper.historyNewestFirst(props.group),
  );

  const columns = computed<DataTableColumns<ProtectionGroupEvent>>(() => [
    {
      title: t('colChange'),
      key: 'at',
      render: (event) => formatDateTime(event.at),
    },
    {
      title: t('switchover'),
      key: 'line',
      render: (event) =>
        t('historyLine', {
          from: event.from || EMPTY_VALUE,
          to: event.to || EMPTY_VALUE,
        }),
    },
    {
      title: t('colMode'),
      key: 'mode',
      render: (event) =>
        h(StatusBadge, {
          tone: event.automatic ? 'warning' : 'info',
          label: event.automatic ? t('historyAutomatic') : t('historyManual'),
        }),
    },
    {
      title: t('colMemberLag'),
      key: 'meta',
      render: (event) =>
        t('historyMeta', {
          generation: event.generation ?? 0,
          lag: event.eventLag ?? 0,
        }),
    },
  ]);
</script>

<template>
  <div class="activity-tab">
    <section
      v-if="isSwitchActive"
      class="activity-tab__progress"
    >
      <h2 class="activity-tab__title">
        {{
          group.target
            ? t('progressTitle', { target: group.target })
            : t('progressGeneric')
        }}
      </h2>
      <ol class="activity-tab__steps">
        <li
          v-for="(step, idx) in steps"
          :key="step.id"
          class="activity-tab__step"
          :class="`activity-tab__step--${step.status}`"
        >
          <span class="activity-tab__index">
            {{ step.status === 'done' ? '✓' : idx + 1 }}
          </span>
          {{ t(`step.${step.id}`) }}
        </li>
      </ol>
    </section>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="false"
      :row-key="(e: ProtectionGroupEvent) => `${e.at ?? ''}-${e.to ?? ''}`"
    >
      <template #empty>
        <p class="activity-tab__empty">{{ t('historyEmpty') }}</p>
      </template>
    </DataTable>
  </div>
</template>

<style lang="scss" scoped>
  .activity-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__title {
      margin: 0 0 var(--tt-space-3);
      font-size: var(--tt-text-md);
      font-weight: var(--tt-weight-strong);
    }

    &__empty {
      margin: 0;
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }

    &__steps {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: var(--tt-space-3);
      margin: 0;
      padding: 0;
      list-style: none;
    }

    &__step {
      display: flex;
      align-items: center;
      gap: var(--tt-space-2);
      padding: var(--tt-space-2) var(--tt-space-3);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);

      &--done {
        border-color: var(--tt-success-border);
        background-color: var(--tt-success-bg);
        color: var(--tt-success);
      }

      &--active {
        border-color: var(--tt-info-border);
        background-color: var(--tt-info-bg);
        color: var(--tt-info);
        font-weight: var(--tt-weight-strong);
      }
    }

    &__index {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 22px;
      height: 22px;
      border: 1px solid currentcolor;
      border-radius: 50%;
      font-size: var(--tt-text-xs);
    }
  }
</style>
