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
  import { CInput, CSkeleton } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { useTuttiDiffReportEntriesStore } from '@/stores/tuttiDiffReportEntriesStore';
  import i18nDiffReportDetail from '@/components/tutti/diff-report-detail/i18nDiffReportDetail';
  import { useTuttiDiffReportDetailStore } from '@/stores/tuttiDiffReportDetailStore';

  const { t } = useI18n({ messages: i18nDiffReportDetail });
  const { filterObjectName, isLoading } = storeToRefs(
    useTuttiDiffReportEntriesStore(),
  );
  const { report } = storeToRefs(useTuttiDiffReportDetailStore());
</script>

<template>
  <div class="diff-report-detail-entries-filters">
    <div
      v-if="!report || !report.ready || isLoading"
      key="loading"
      class="diff-report-detail-entries-filters__list"
    >
      <CSkeleton
        :height="34"
        :border-radius="4"
      />
    </div>
    <div
      v-else
      key="filters"
      class="diff-report-detail-entries-filters__list"
    >
      <CInput
        v-model:value="filterObjectName"
        :placeholder="t('entriesSearchPlaceholder')"
        clearable
        class="diff-report-detail-entries-filters__search"
      />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .diff-report-detail-entries-filters {
    &__list {
      max-width: 420px;
    }
  }
</style>
