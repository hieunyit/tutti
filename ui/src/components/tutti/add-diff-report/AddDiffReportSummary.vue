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
  /** Running summary next to the steps: source and destination as storage / bucket. */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import i18nAddDiffReport from './i18nAddDiffReport';
  import { useTuttiAddDiffReportStore } from '@/stores/tuttiAddDiffReportStore';

  const { t } = useI18n({ messages: i18nAddDiffReport });
  const { fromStorage, fromBucketName, toStorage, toBucketName } = storeToRefs(
    useTuttiAddDiffReportStore(),
  );

  const place = (storage: string | undefined, bucket: string) =>
    [storage, bucket].filter(Boolean).join(' / ') || '–';

  const rows = computed(() => [
    {
      label: t('summarySource'),
      value: place(fromStorage.value?.name, fromBucketName.value),
    },
    {
      label: t('summaryDestination'),
      value: place(toStorage.value?.name, toBucketName.value),
    },
  ]);
</script>

<template>
  <dl class="diff-summary">
    <div
      v-for="row in rows"
      :key="row.label"
    >
      <dt>{{ row.label }}</dt>
      <dd>{{ row.value }}</dd>
    </div>
  </dl>
</template>

<style scoped lang="scss">
  .diff-summary {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    margin: 0;

    dt {
      color: var(--tt-text-2);
      font-size: var(--tt-text-sm);
    }

    dd {
      margin: 0;
      font-weight: var(--tt-weight-strong);
      overflow-wrap: anywhere;
    }
  }
</style>
