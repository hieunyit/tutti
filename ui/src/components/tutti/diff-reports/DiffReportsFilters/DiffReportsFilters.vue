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
  import { NFormItem } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import DiffReportsFilterByBucket from '@/components/tutti/diff-reports/DiffReportsFilterByBucket/DiffReportsFilterByBucket.vue';
  import DiffReportsFilterByDirection from '@/components/tutti/diff-reports/DiffReportsFilterByDirection/DiffReportsFilterByDirection.vue';
  import DiffReportsFilterByStatus from '@/components/tutti/diff-reports/DiffReportsFilterByStatus/DiffReportsFilterByStatus.vue';
  import i18nDiffReports from '@/components/tutti/diff-reports/i18nDiffReports';

  const { t } = useI18n({ messages: i18nDiffReports });
</script>

<template>
  <div class="diff-reports-filters">
    <NFormItem
      :label="t('filterLabelStatus')"
      :show-feedback="false"
    >
      <DiffReportsFilterByStatus />
    </NFormItem>
    <NFormItem
      :label="t('filterLabelStorage')"
      :show-feedback="false"
    >
      <DiffReportsFilterByDirection />
    </NFormItem>
    <NFormItem
      :label="t('filterLabelBucket')"
      :show-feedback="false"
    >
      <DiffReportsFilterByBucket />
    </NFormItem>
  </div>
</template>

<style lang="scss" scoped>
  .diff-reports-filters {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: var(--tt-space-2) var(--tt-space-3);
    width: 100%;
  }
</style>
