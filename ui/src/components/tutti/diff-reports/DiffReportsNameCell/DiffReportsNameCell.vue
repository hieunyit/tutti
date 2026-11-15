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
   * Primary text of a diff report row: a link to its detail page that reads
   * "storage/bucket → storage/bucket". Reports made with the CLI have no two
   * locations and show a short note instead.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import i18nDiffReports from '@/components/tutti/diff-reports/i18nDiffReports';
  import { diffDetailRoute } from '@/pages/TuttiReplicationPage/diffRoutes';
  import { DiffReportsHelper } from '@/utils/helpers/DiffReportsHelper';
  import type { DiffReport } from '@/utils/types/tuttiCore';

  const props = defineProps<{ report: DiffReport }>();

  const { t } = useI18n({ messages: i18nDiffReports });

  const isTwo = computed(() =>
    DiffReportsHelper.isTwoLocationReport(props.report),
  );
  const label = computed(() =>
    props.report.locations
      .map((location) => `${location.storage}/${location.bucket}`)
      .join(' → '),
  );
  const to = computed(() => {
    const [from, dest] = props.report.locations;

    return from && dest ? diffDetailRoute(from, dest) : undefined;
  });
</script>

<template>
  <RouterLink
    v-if="isTwo && to"
    class="diff-reports-name"
    :to="to"
    :title="label"
    >{{ label }}</RouterLink
  >
  <span
    v-else
    class="diff-reports-name diff-reports-name--plain"
    >{{ t('cliReport') }}</span
  >
</template>

<style lang="scss" scoped>
  .diff-reports-name {
    display: inline-block;
    max-width: 100%;
    overflow-wrap: anywhere;
    color: var(--tt-accent);
    font-weight: var(--tt-weight-strong);
    text-decoration: none;

    &:hover {
      color: var(--tt-accent-hover);
      text-decoration: underline;
    }

    &:focus-visible {
      outline: 2px solid var(--tt-accent);
      outline-offset: 2px;
    }

    &--plain {
      color: var(--tt-text-3);
      font-weight: var(--tt-weight-regular, 400);
    }
  }
</style>
