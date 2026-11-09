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
  import { useI18n } from 'vue-i18n';
  import ListCard from '@/components/tutti/common/ListCard.vue';
  import i18nDiffReports from '@/components/chorus/diff-reports/i18nDiffReports';
  import DiffReportsList from '@/components/chorus/diff-reports/DiffReportsList/DiffReportsList.vue';
  import DiffReportsFilters from '@/components/chorus/diff-reports/DiffReportsFilters/DiffReportsFilters.vue';
  import DiffReportsListActions from '@/components/chorus/diff-reports/DiffReportsListActions/DiffReportsListActions.vue';
  import DiffReportsListTags from '@/components/chorus/diff-reports/DiffReportsListTags/DiffReportsListTags.vue';

  const { t } = useI18n({ messages: i18nDiffReports });
</script>

<template>
  <ListCard :title="t('diffListTitle')">
    <template #filters>
      <DiffReportsFilters />
    </template>
    <template #tags>
      <DiffReportsListTags />
    </template>
    <template #actions>
      <DiffReportsListActions />
    </template>

    <DiffReportsList />
  </ListCard>
</template>
