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
  import { computed } from 'vue';
  import i18nDiffReports from '@/components/tutti/diff-reports/i18nDiffReports';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import type { DiffReport } from '@/utils/types/tuttiCore';

  const props = defineProps<{
    report: DiffReport;
  }>();

  const { t } = useI18n({
    messages: i18nDiffReports,
  });

  const status = computed(() => {
    if (!props.report.ready) {
      return { variant: 'warning' as const, label: t('statusChecking') };
    }

    return props.report.consistent
      ? { variant: 'success' as const, label: t('statusConsistent') }
      : { variant: 'danger' as const, label: t('statusInconsistent') };
  });
</script>

<template>
  <div class="diff-reports-status-cell">
    <StatusTag
      :variant="status.variant"
      :label="status.label"
    />
  </div>
</template>
