<!--
  - Copyright © 2026 Clyso GmbH
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
  /**
   * One status badge; the check progress shows only while the report is
   * still being checked.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { StatusBadge, type StatusTone } from '@/components/kit';
  import i18nDiffReports from '@/components/tutti/diff-reports/i18nDiffReports';
  import type { DiffReport } from '@/utils/types/tuttiCore';

  const props = defineProps<{ report: DiffReport }>();

  const { t } = useI18n({ messages: i18nDiffReports });

  const status = computed<{ tone: StatusTone; label: string }>(() => {
    if (!props.report.ready)
      return { tone: 'info', label: t('statusChecking') };

    return props.report.consistent
      ? { tone: 'success', label: t('statusConsistent') }
      : { tone: 'danger', label: t('statusInconsistent') };
  });

  const percent = computed<number | null>(() => {
    const queued = Number(props.report.queued);

    if (props.report.ready || queued <= 0) return null;

    return Math.min(
      99,
      Math.floor((Number(props.report.completed) / queued) * 100),
    );
  });
</script>

<template>
  <div class="diff-reports-status">
    <StatusBadge
      :tone="status.tone"
      :label="status.label"
    />
    <span
      v-if="percent !== null"
      class="diff-reports-status__progress"
      >{{ t('checkedPercent', { n: percent }) }}</span
    >
  </div>
</template>

<style lang="scss" scoped>
  .diff-reports-status {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;

    &__progress {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }
  }
</style>
