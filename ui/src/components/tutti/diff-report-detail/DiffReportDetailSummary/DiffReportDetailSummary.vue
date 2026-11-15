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
  /**
   * Summary of a diff report, once: check progress, how many objects differ,
   * how the buckets are compared, and the fix progress when a fix ran. The
   * status itself lives in the page header.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import TuttiDiffReportsProgress from '@/components/tutti/common/TuttiDiffReportProgress/TuttiDiffReportsProgress.vue';
  import i18nDiffReportDetail from '@/components/tutti/diff-report-detail/i18nDiffReportDetail';
  import { useTuttiDiffReportDetailStore } from '@/stores/tuttiDiffReportDetailStore';
  import { useTuttiDiffReportEntriesStore } from '@/stores/tuttiDiffReportEntriesStore';

  const { t } = useI18n({ messages: i18nDiffReportDetail });
  const { report, hasFixActivity } = storeToRefs(
    useTuttiDiffReportDetailStore(),
  );
  const { inconsistentObjectsCount, hasError, isLoading } = storeToRefs(
    useTuttiDiffReportEntriesStore(),
  );

  const differing = computed(() =>
    hasError.value || isLoading.value ? '-' : inconsistentObjectsCount.value,
  );

  // only what departs from the default comparison is worth saying
  const settings = computed(() => {
    const current = report.value;

    if (!current) return '';

    return [
      current.versioned ? t('configVersioned') : t('configNotVersioned'),
      current.ignoreEtags ? t('configIgnoresEtags') : t('configConsidersEtags'),
      current.ignoreSizes ? t('configIgnoresSizes') : t('configConsidersSizes'),
    ].join(' · ');
  });
</script>

<template>
  <dl
    v-if="report"
    class="diff-summary"
  >
    <div class="diff-summary__row">
      <dt>{{ t('progressLabel') }}</dt>
      <dd>
        <TuttiDiffReportsProgress :report="report" />
        <span>{{
          t('progressCount', { done: report.completed, total: report.queued })
        }}</span>
      </dd>
    </div>
    <div class="diff-summary__row">
      <dt>{{ t('progressInconsistentObjects') }}</dt>
      <dd>{{ differing }}</dd>
    </div>
    <div class="diff-summary__row">
      <dt>{{ t('overviewComparison') }}</dt>
      <dd>{{ settings }}</dd>
    </div>
    <div
      v-if="hasFixActivity"
      class="diff-summary__row"
    >
      <dt>{{ t('fixProgressTitle') }}</dt>
      <dd>
        <TuttiDiffReportsProgress
          :report="report"
          type="fix"
        />
        <span>{{
          t('progressCount', {
            done: report.fixCompleted,
            total: report.fixQueued,
          })
        }}</span>
      </dd>
    </div>
  </dl>
</template>

<style lang="scss" scoped>
  .diff-summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--tt-space-4);
    margin: 0;

    &__row {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      min-width: 0;
    }

    dt {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }

    dd {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      margin: 0;
      overflow-wrap: anywhere;
    }
  }
</style>
